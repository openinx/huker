package core

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"reflect"
	"sort"
	"testing"
)

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

	actualMap := utils.MergeMap(m1, m1)
	if !reflect.DeepEqual(m1, actualMap) {
		t.Errorf("Merge the same yaml map failed. expected: %v, actual: %v", m1, actualMap)
	}

	actualMap = utils.MergeMap(m1, m2)
	if !reflect.DeepEqual(m3, actualMap) {
		t.Errorf("Merge the m1 with m2 failed, expected: %v, actual: %v", m3, actualMap)
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
	if a.JobName != b.JobName {
		return fmt.Errorf("jobName mismatch, expected: %s, actual: %s", a.JobName, b.JobName)
	}
	if err := assertSliceEquals(a.JvmOpts, b.JvmOpts, "jvm_opts"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.JvmProperties, b.JvmProperties, "jvm_properties"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.Classpath, b.Classpath, "classpath"); err != nil {
		return err
	}
	if err := assertSliceEquals(a.MainEntry.toShell(), b.MainEntry.toShell(), "mainEntry"); err != nil {
		return fmt.Errorf("%s != %s", a.MainEntry.toShell(), b.MainEntry.toShell())
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
      project: zookeeper
      cluster_name: tst-cluster
      main_process: /usr/bin/java
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
		{
			BaseConfig:    "/home/test.yaml",
			Project:       "zookeeper",
			ClusterName:   "tst-cluster",
			MainProcess:   "/usr/bin/java",
			PackageName:   "zookeeper-3.4.11.tar.gz",
			PackageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			Jobs: map[string]*Job{
				"zookeeper": {
					JobName: "zookeeper",
					//hosts:         []string{"192.168.0.1"},
					JvmOpts:       []string{"-Xmx4096m"},
					JvmProperties: []string{"java.log.dir=."},
					Classpath:     []string{"./*"},
					ConfigFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					MainEntry: &MainEntry{
						JavaClass: "org.apache.zookeeper.server.quorum.QuorumPeerMain",
						ExtraArgs: "conf/zoo_sample.cfg",
					},
				},
			},
		},
		{
			BaseConfig:    "/home/test.yaml",
			Project:       "zookeeper",
			ClusterName:   "tst-cluster",
			MainProcess:   "/usr/bin/java",
			PackageName:   "zookeeper-3.4.11.tar.gz",
			PackageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			Jobs: map[string]*Job{
				"zookeeper": {
					JobName: "zookeeper",
					//hosts:         []string{"192.168.0.1", "192.168.0.2"},
					JvmOpts:       []string{"-Xmn1024m"},
					JvmProperties: []string{"java.log.file=/home/log/zk.log"},
					Classpath:     []string{},
					ConfigFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					MainEntry: &MainEntry{},
				},
			},
		},
		{
			BaseConfig:    "/home/test.yaml",
			Project:       "zookeeper",
			ClusterName:   "tst-cluster",
			MainProcess:   "/usr/bin/java",
			PackageName:   "zookeeper-3.4.11.tar.gz",
			PackageMd5sum: "55aec6196ed9fa4c451cb5ae4a1f42d8",
			Jobs: map[string]*Job{
				"zookeeper": {
					JobName: "zookeeper",
					//hosts:         []string{"192.168.0.1", "192.168.0.2"},
					JvmOpts:       []string{"-Xmn1024m", "-Xmx4096m"},
					JvmProperties: []string{"java.log.file=/home/log/zk.log", "java.log.dir=."},
					Classpath:     []string{"./*"},
					ConfigFiles: map[string]ConfigFile{
						"zoo.cfg": INIConfigFile{
							cfgName:   "zoo.cfg",
							keyValues: []string{"data_dir=/home/data", "tick_time=2000"},
						},
					},
					MainEntry: &MainEntry{
						JavaClass: "org.apache.zookeeper.server.quorum.QuorumPeerMain",
						ExtraArgs: "conf/zoo_sample.cfg",
					},
				},
			},
		},
	}

	for i := range testSet {
		if s, err := NewCluster(testSet[i], nil); err != nil {
			t.Errorf("test case #%d failed, cause: %v", i, err)
		} else {
			if s.BaseConfig != expected[i].BaseConfig {
				t.Errorf("test case #%d failed, base mismatch", i)
			}
			if s.Project != expected[i].Project {
				t.Errorf("test case #%d failed, project mismatch", i)
			}
			if s.ClusterName != expected[i].ClusterName {
				t.Errorf("test case #%d failed, cluster_name mismatch", i)
			}
			if s.MainProcess != expected[i].MainProcess {
				t.Errorf("test case #%d failed, main_process mismatch", i)
			}
			if s.PackageName != expected[i].PackageName {
				t.Errorf("test case #%d failed, package_name mismatch", i)
			}
			if s.PackageMd5sum != expected[i].PackageMd5sum {
				t.Errorf("test case #%d failed, package_md5sum mismatch", i)
			}
			if len(s.Jobs) != len(expected[i].Jobs) {
				t.Errorf("test case #%d failed, job size mismatch", i)
			}
			for k := range s.Jobs {
				if s.Jobs[k].SuperJob != "" {
					t.Errorf("test case #%d, superJob should be empty by default.", i)
				}
				if err := assertJobEquals(s.Jobs[k], expected[i].Jobs[k]); err != nil {
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
      project: zookeeper
      cluster_name: tst-cluster
      main_process: /usr/bin/java
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
