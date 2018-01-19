package huker

import (
	"reflect"
	"testing"
)

func TestConfigFile(t *testing.T) {
	type TestCase struct {
		cfgName       string
		inputs        []string
		keyValues     map[string]string
		configuration string
	}

	testCase := []TestCase{
		{"zoo0.cfg", []string{}, map[string]string{}, ""},
		{"zoo1.cfg", []string{"a=b", "c=d"}, map[string]string{"a": "b", "c": "d"}, "a=b\nc=d"},
		{"zoo2.properties", []string{"a=b", "c=d"}, map[string]string{"a": "b", "c": "d"}, "a=b\nc=d"},
		{"test0.xml", []string{"a=b"}, map[string]string{"a": "b"},
			"<configuration>\n  <property>\n    <name>a</name>\n    <value>b</value>\n  </property>\n</configuration>"},
		{"test1.xml", []string{}, map[string]string{}, "<configuration>\n</configuration>"},
		{"myid", []string{"1"}, map[string]string{"0": "1"}, "1"},
		{"myid.txt", []string{"1"}, map[string]string{"0": "1"}, "1"},
		{"myid", []string{"1", "2"}, map[string]string{"0": "1", "1": "2"}, "1\n2"},
	}

	for caseId, cas := range testCase {
		cf, err := ParseConfigFile(cas.cfgName, cas.inputs)
		if err != nil {
			t.Errorf("TestCase #%d failed, cause: %v", caseId, err)
		}
		if cf.getConfigName() != cas.cfgName {
			t.Errorf("TestCase #%d failed, `%s` != `%s`", caseId, cf.getConfigName(), cas.cfgName)
		}
		if !reflect.DeepEqual(cf.toKeyValue(), cas.keyValues) {
			t.Errorf("TestCase #%d failed, `%v` != `%v`", caseId, cf.toKeyValue(), cas.keyValues)
		}
		if cf.toString() != cas.configuration {
			t.Errorf("TestCase #%d failed, `%s` != `%s`", caseId, cf.toString(), cas.configuration)
		}

	}
}

func TestMergeWith(t *testing.T) {
	// TODO unit test for this.
}
