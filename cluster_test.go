package huker

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func testMapEquals(m1 map[interface{}]interface{}, m2 map[interface{}]interface{}) bool {
	if m1 == nil && m2 == nil {
		return true
	}
	if m1 == nil || m2 == nil {
		return false
	}
	if len(m1) != len(m2) {
		return false
	}
	for key := range m1 {
		value := m1[key]
		if value == nil {
			if m2[key] == nil {
				continue
			} else {
				return false
			}
		}
		if m2[key] == nil {
			return false
		}
		switch reflect.TypeOf(value).Kind() {
		case reflect.Map:
			if !testMapEquals(m1[key].(map[interface{}]interface{}), m2[key].(map[interface{}]interface{})) {
				return false
			}
		case reflect.Slice:
			a1 := m1[key].([]interface{})
			a2 := m2[key].([]interface{})
			if len(a1) != len(a2) {
				return false
			}
			for i := range a1 {
				if a1[i] != a2[i] {
					return false
				}
			}
		default:
			if value != m2[key] {
				return false
			}
		}
	}
	return true
}

func TestMergeYamlMap(t *testing.T) {
	m1 := map[interface{}]interface{}{
		"jobs": map[interface{}]interface{}{
			"zookeeper": "test-cluster",
			"hosts":     []interface{}{"a1", "b1"},
		},
		"tasks": []interface{}{"a", "b"},
	}

	m2 := map[interface{}]interface{}{
		"jobs": map[interface{}]interface{}{
			"zookeeper": "test-cluster2",
			"hosts":     []interface{}{"a3"},
		},
		"tasks":   []interface{}{"c", "b"},
		"compute": "d",
	}

	m3 := map[interface{}]interface{}{
		"jobs": map[interface{}]interface{}{
			"zookeeper": "test-cluster",
			"hosts":     []interface{}{"a1", "b1", "a3"},
		},
		"tasks":   []interface{}{"a", "b", "c"},
		"compute": "d",
	}

	actualMap := MergeMap(m1, m1)
	if !testMapEquals(m1, actualMap) {
		t.Errorf("Merge the same yaml map failed. expected: %v, actual: %v", m1, actualMap)
	}

	actualMap = MergeMap(m1, m2)
	if !testMapEquals(m3, actualMap) {
		t.Errorf("Merge the m1 with m2 failed, expeced: %v, actual: %v", m3, actualMap)
	}
}

func assertSliceEquals(a, b []string, key string) error {
	if len(a) != len(b) {
		return fmt.Errorf("%s length not equals(%d != %d): %v != %v", key, len(a), len(b), a, b)
	}
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return fmt.Errorf("%s: %s != %s", key, a[i], b[i])
		}
	}
	return nil
}

func assertJobEquals(a, b *Job) error {
	if a.jobName != b.jobName {
		return fmt.Errorf("jobName mismatch, expected: %s, actual: %s", a.jobName, b.jobName)
	}
	if err := assertSliceEquals(a.jvmOpts, b.jvmOpts, "jvm_opts"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.jvmProperties, b.jvmProperties, "jvm_properties"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.classpath, b.classpath, "classpath"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.mainEntry.toShell(), b.mainEntry.toShell(), "mainEntry"); err != nil {
		return fmt.Errorf("%s != %s", a.mainEntry.toShell(), b.mainEntry.toShell())
	}
	return nil
}

func TestNewServiceConfig(t *testing.T) {
	cfg0 := `
    jobs:
      zookeeper:
        jvm_opts:
          - -Xmx4096m
        jvm_properties:
          - java.log.dir=.
        config:
          zoo.cfg:
            - data_dir=/home/data
        classpath:
          - ./*
        main_entry:
          java_class: org.apache.zookeeper.server.quorum.QuorumPeerMain
          extra_args: conf/zoo_sample.cfg
    `

	cfg1 := `
    base: /home/test.yaml
    cluster:
      cluster_name: tst-cluster
      java_home: /usr/bin/java
      package_name: zookeeper-3.4.11.tar.gz
      package_md5sum: 55aec6196ed9fa4c451cb5ae4a1f42d8
    jobs:
      zookeeper:
        hosts:
          - 192.168.0.2:9001/id=1/base_port=9002
        config:
          zoo.cfg:
            - tick_time=2000
    `

	cfg2 := `
    jobs:
      zookeeper:
        hosts:
          - 192.168.0.2:9001/id=1/base_port=9003
        jvm_opts:
          - -Xmn1024m
        jvm_properties:
          - java.log.file=/home/log/zk.log
    `

	cfgList0 := []string{cfg1, cfg0}
	cfgList1 := []string{cfg1, cfg2}
	cfgList2 := []string{cfg2, cfg1, cfg0}

	testSet := [][]string{
		cfgList0, cfgList1, cfgList2,
	}

	expected := []Cluster{
		Cluster{
			baseConfig:    "/home/test.yaml",
			clusterName:   "tst-cluster",
			javaHome:      "/usr/bin/java",
			packageName:   "zookeeper-3.4.11.tar.gz",
			packageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			jobs: map[string]*Job{
				"zookeeper": &Job{
					jobName: "zookeeper",
					//hosts:         []string{"192.168.0.1"},
					jvmOpts:       []string{"-Xmx4096m"},
					jvmProperties: []string{"java.log.dir=."},
					classpath:     []string{"./*"},
					configFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					mainEntry: &MainEntry{
						javaClass: "org.apache.zookeeper.server.quorum.QuorumPeerMain",
						extraArgs: "conf/zoo_sample.cfg",
					},
				},
			},
		},
		Cluster{
			baseConfig:    "/home/test.yaml",
			clusterName:   "tst-cluster",
			javaHome:      "/usr/bin/java",
			packageName:   "zookeeper-3.4.11.tar.gz",
			packageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			jobs: map[string]*Job{
				"zookeeper": &Job{
					jobName: "zookeeper",
					//hosts:         []string{"192.168.0.1", "192.168.0.2"},
					jvmOpts:       []string{"-Xmn1024m"},
					jvmProperties: []string{"java.log.file=/home/log/zk.log"},
					classpath:     []string{},
					configFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					mainEntry: &MainEntry{},
				},
			},
		},
		Cluster{
			baseConfig:    "/home/test.yaml",
			clusterName:   "tst-cluster",
			javaHome:      "/usr/bin/java",
			packageName:   "zookeeper-3.4.11.tar.gz",
			packageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			jobs: map[string]*Job{
				"zookeeper": &Job{
					jobName: "zookeeper",
					//hosts:         []string{"192.168.0.1", "192.168.0.2"},
					jvmOpts:       []string{"-Xmn1024m", "-Xmx4096m"},
					jvmProperties: []string{"java.log.file=/home/log/zk.log", "java.log.dir=."},
					classpath:     []string{"./*"},
					configFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					mainEntry: &MainEntry{
						javaClass: "org.apache.zookeeper.server.quorum.QuorumPeerMain",
						extraArgs: "conf/zoo_sample.cfg",
					},
				},
			},
		},
	}

	for i := range testSet {
		if s, err := NewCluster(testSet[i], nil); err != nil {
			t.Errorf("test case #%d failed, cause: %v", i, err)
		} else {
			if s.baseConfig != expected[i].baseConfig {
				t.Errorf("test case #%d failed, base mismatch", i)
			}
			if s.clusterName != expected[i].clusterName {
				t.Errorf("test case #%d failed, cluster_name mismatch", i)
			}
			if s.javaHome != expected[i].javaHome {
				t.Errorf("test case #%d failed, java_home mismatch", i)
			}
			if s.packageName != expected[i].packageName {
				t.Errorf("test case #%d failed, package_name mismatch", i)
			}
			if s.packageMd5sum != expected[i].packageMd5sum {
				t.Errorf("test case #%d failed, package_md5sum mismatch", i)
			}
			if len(s.jobs) != len(expected[i].jobs) {
				t.Errorf("test case #%d failed, job size mismatch", i)
			}
			for k := range s.jobs {
				if s.jobs[k].superJob != "" {
					t.Errorf("test case #%d, superJob should be empty by default.", i)
				}
				if err := assertJobEquals(s.jobs[k], expected[i].jobs[k]); err != nil {
					t.Errorf("test case #%d failed, cause: %v", i, err)
				}
			}
		}
	}
}

func TestToShell(t *testing.T) {
	cfg := `
    base: /home/test.yaml
    cluster:
      cluster_name: tst-cluster
      java_home: /usr/bin/java
      package_name: zookeeper-3.4.11.tar.gz
      package_md5sum: 55aec6196ed9fa4c451cb5ae4a1f42d8
    jobs:
      zookeeper:
        hosts:
          - 192.168.0.2:9001/id=1/base_port=9003
        jvm_properties:
          - java.log.dir=.
        config:
          zoo.cfg:
            - data_dir=/home/data
        classpath:
          - ./*
        main_entry:
          java_class: org.apache.zk.QuorumPeerMain
          extra_args: conf/zoo_sample.cfg
    `
	s, err := NewCluster([]string{cfg}, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"/usr/bin/java",
		"-Djava.log.dir=.",
		"-cp",
		"./*",
		"org.apache.zk.QuorumPeerMain",
		"conf/zoo_sample.cfg",
	}
	if err := assertSliceEquals(s.toShell("zookeeper"), expected, "Shell"); err != nil {
		t.Errorf("%v", err)
	}
}
