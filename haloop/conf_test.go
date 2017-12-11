package haloop

import (
    "testing"
    "reflect"
    "fmt"
    "sort"
    "github.com/qiniu/log"
)

func TestConfigFile(t *testing.T) {
    var cf ConfigFile
    cf = NewINIConfigFile("test.cfg", []string{
        "a=b", "c=d",
    })
    if cf.toString() != "a=b\nc=d" {
        t.Error()
    }
    m := cf.toKeyValue()
    for _, x := range []string{"a", "c"} {
        if _, ok := m[x]; !ok {
            t.Error()
        }
    }
    if len(m) != 2 {
        t.Error()
    }

    cf = NewINIConfigFile("test.cfg", []string{})
    if cf.toString() != "" {
        t.Error()
    }
    if len(cf.toKeyValue()) != 0 {
        t.Error()
    }

    xml := NewXMLConfigFile("test.xml", []string{"a=b"})
    expected := "<configuration>\n  <property>\n    <name>a</name>\n    <value>b</value>\n  </property>\n</configuration>"
    if xml.toString() != expected {
        t.Error(xml.toString())
    }
    m = xml.toKeyValue()
    for _, x := range []string{"a"} {
        if _, ok := m[x]; !ok {
            t.Errorf("%s does not exists", x)
        }
    }

    xml = NewXMLConfigFile("test.xml", []string{})
    expected = "<configuration>\n</configuration>"
    if xml.toString() != expected {
        t.Error(xml.toString())
    }

}

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
            }else {
                return false
            }
        }
        if m2[key] == nil {
            return false
        }
        switch  reflect.TypeOf(value).Kind(){
        case reflect.Map:
            if !testMapEquals(m1[key].(map[interface{}]interface{}), m2[key].(map[interface{}]interface{})) {
                return false;
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
            "zookeeper" : "test-cluster",
            "hosts": []interface{}{"a1", "b1"},
        },
        "tasks": []interface{}{"a", "b"},
    }

    m2 := map[interface{}]interface{}{
        "jobs": map[interface{}]interface{}{
            "zookeeper": "test-cluster2",
            "hosts": []interface{}{"a3"},
        },
        "tasks": []interface{}{"c", "b"},
        "compute": "d",
    }

    m3 := map[interface{}]interface{}{
        "jobs": map[interface{}]interface{}{
            "zookeeper": "test-cluster",
            "hosts": []interface{}{"a1", "b1", "a3"},
        },
        "tasks": []interface{}{"a", "b", "c"},
        "compute": "d",
    }

    actualMap := MergeYamlMap(m1, m1)
    if !testMapEquals(m1, actualMap) {
        t.Errorf("Merge the same yaml map failed. expected: %v, actual: %v", m1, actualMap)
    }

    actualMap = MergeYamlMap(m1, m2)
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

func assertJobEquals(a, b Job) error {
    if a.jobName != b.jobName {
        return fmt.Errorf("jobName mismatch, expected: %s, actual: %s", a.jobName, b.jobName)
    }
    if a.basePort != b.basePort {
        return fmt.Errorf("basePort mismatch, expected: %d, actual: %d", a.basePort, b.basePort)
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
        base_port: 9010
        hosts:
          - 192.168.0.1
        config:
          zoo.cfg:
            - tick_time=2000
    `

    cfg2 := `
    jobs:
      zookeeper:
        base_port: 9012
        hosts:
          - 192.168.0.2
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

    expected := []ServiceConfig{
        ServiceConfig{
            baseConfig:"/home/test.yaml",
            clusterName:"tst-cluster",
            javaHome:"/usr/bin/java",
            packageName:"zookeeper-3.4.11.tar.gz",
            packageMd5sum:"55aec6196ed9fa4c451cb5ae4a1f42d8",
            jobs:map[string]Job{
                "zookeeper":Job{
                    jobName:"zookeeper",
                    basePort:9010,
                    hosts:[]string{"192.168.0.1"},
                    jvmOpts:[]string{"-Xmx4096m"},
                    jvmProperties:[]string{"java.log.dir=."},
                    classpath:[]string{"./*"},
                    configFiles:map[string]ConfigFile{
                        "zoo.cfg": INIConfigFile{
                            cfgName:"zoo.cfg",
                            keyValues:[]string{"data_dir=/home/data", "tick_time=2000"},
                        },
                    },
                    mainEntry:MainEntry{
                        javaClass:"org.apache.zookeeper.server.quorum.QuorumPeerMain",
                        extraArgs:"conf/zoo_sample.cfg",
                    },
                },

            },
        },
        ServiceConfig{
            baseConfig:"/home/test.yaml",
            clusterName:"tst-cluster",
            javaHome:"/usr/bin/java",
            packageName:"zookeeper-3.4.11.tar.gz",
            packageMd5sum:"55aec6196ed9fa4c451cb5ae4a1f42d8",
            jobs:map[string]Job{
                "zookeeper":Job{
                    jobName:"zookeeper",
                    basePort:9010,
                    hosts:[]string{"192.168.0.1", "192.168.0.2"},
                    jvmOpts:[]string{"-Xmn1024m"},
                    jvmProperties:[]string{"java.log.file=/home/log/zk.log"},
                    classpath:[]string{},
                    configFiles:map[string]ConfigFile{
                        "zoo.cfg": INIConfigFile{
                            cfgName:"zoo.cfg",
                            keyValues:[]string{"data_dir=/home/data", "tick_time=2000"},
                        },
                    },
                    mainEntry:MainEntry{},
                },

            },
        },
        ServiceConfig{
            baseConfig:"/home/test.yaml",
            clusterName:"tst-cluster",
            javaHome:"/usr/bin/java",
            packageName:"zookeeper-3.4.11.tar.gz",
            packageMd5sum:"55aec6196ed9fa4c451cb5ae4a1f42d8",
            jobs:map[string]Job{
                "zookeeper":Job{
                    jobName:"zookeeper",
                    basePort:9012,
                    hosts:[]string{"192.168.0.1", "192.168.0.2"},
                    jvmOpts:[]string{"-Xmn1024m", "-Xmx4096m"},
                    jvmProperties:[]string{"java.log.file=/home/log/zk.log", "java.log.dir=."},
                    classpath:[]string{"./*"},
                    configFiles:map[string]ConfigFile{
                        "zoo.cfg": INIConfigFile{
                            cfgName:"zoo.cfg",
                            keyValues:[]string{"data_dir=/home/data", "tick_time=2000"},
                        },
                    },
                    mainEntry:MainEntry{
                        javaClass:"org.apache.zookeeper.server.quorum.QuorumPeerMain",
                        extraArgs:"conf/zoo_sample.cfg",
                    },
                },

            },
        },
    }

    for i := range testSet {
        if s, err := NewServiceConfig(testSet[i]); err != nil {
            t.Errorf("test case #%d failed, cause: %v", i, err)
        }else {
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
        base_port: 9010
        hosts:
          - 192.168.0.1
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
    s, err := NewServiceConfig([]string{cfg})
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

func TestLoadServiceConfig(t *testing.T) {
    e := &EnvVariables{
        JavaHome:"/usr/bin/java",
        ConfRootDir:"../",
        PkgRootDir:"/Users/openinx/test/zk/pkg/zookeeper-3.4.11",
        PkgConfDir:"/Users/openinx/test/zk/conf",
        PkgDataDir:"/Users/openinx/test/zk/data",
        PkgLogDir:"/Users/openinx/test/zk/log",
        PkgStdoutDir:"/Users/openinx/test/zk/stdout",
    }

    s, err := LoadServiceConfig("/Users/openinx/gopath/src/gitlab.com/openinx/haloop/conf/zookeeper/test-cluster.yaml", e)
    if err != nil {
        t.Errorf("Loading service config error: %s", err)
    }
    log.Info(s.toShell("zookeeper"))
}