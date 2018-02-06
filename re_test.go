package huker

import (
	"regexp"
	"testing"
)

func TestPattern(t *testing.T) {

	var testCases = []struct {
		pattern   string
		text      string
		isMatched bool
	}{
		{patternClusterName, "address=%{cluster.name}", true},
		{patternClusterName, "address=%{clusteraname}", false},
		{patternJobAttribute, "key=%{job01.0.port0}", true},
		{patternJobAttribute, "key=%{job01x0.port0}", false},
		{patternJobAttribute, "key=%{jOb0_1.0.port0}", true},
		{patternJobAttribute, "key=%{job01.0xport0}", false},
		{patternJobAttribute, "key=%{job01.x.port0}", false},
		{patternJobAttributeNumber, "key=%{jOb0_1.0.base_port+1}", true},
		{patternJobAttributeNumber, "key=%{jOb0_1.0.base_por3t+00100}", true},
		{patternJobAttributeNumber, "key=%{jOb0_1.0.base_port}", false},
		{patternDependenciesServerList, "key=%{dependencies.100.jOb0_1.server_list}", true},
		{patternJobServerList, "key=%{aB_0.server_list}", true},
		{patternVarJobAttribute, "key=%{aB_0.x.base_PORT0}", true},
		{patternVarJobAttributeNumber, "key=%{aB_0.x.base_PORT0+1}", true},
	}

	for i := range testCases {
		re := regexp.MustCompile(testCases[i].pattern)
		if testCases[i].isMatched != re.Match([]byte(testCases[i].text)) {
			t.Errorf("Test case #%d failed, regexp mismatched.", i)
		}
	}
}

func TestRender(t *testing.T) {

	host0, _ := NewHost("www.example.com:9001/base_port=7001/id=0")
	host1, _ := NewHost("www.example.com:9001/base_port=8001/id=1")
	root := &Cluster{
		clusterName: "root",
		jobs: map[string]*Job{
			"test_job": {
				jobName:       "test_job",
				superJob:      "test_super_job",
				hosts:         []*Host{host0, host1},
				jvmOpts:       []string{},
				jvmProperties: []string{},
				classpath:     []string{},
				mainEntry:     &MainEntry{},
				configFiles:   make(map[string]ConfigFile),
				hooks:         make(map[string]string),
			},
		},
		dependencies: []*Cluster{},
	}
	c := &Cluster{
		clusterName: "test",
		jobs: map[string]*Job{
			"test_job": {
				jobName:       "test_job",
				superJob:      "test_super_job",
				hosts:         []*Host{host0, host1},
				jvmOpts:       []string{},
				jvmProperties: []string{},
				classpath:     []string{},
				mainEntry:     &MainEntry{},
				configFiles:   make(map[string]ConfigFile),
				hooks:         make(map[string]string),
			},
		},
		dependencies: []*Cluster{root},
	}
	var testCases = []struct {
		input   string
		success bool
		output  string
	}{
		{"key=%{cluster.name}", true, "key=test"},
		{"key=%{invalid_job.0.host}", false, ""},
		{"key=%{test_job.99999999999999.host}", false, ""},
		{"key=%{test_job.0.invalid_host}", false, ""},
		{"key=%{test_job.0.host}", true, "key=www.example.com"},
		{"key=%{test_job.1.host}", true, "key=www.example.com"},
		{"key=%{test_job.2.host}", false, ""},
		{"key=%{invalid_job.0.base_port+100}", false, ""},
		{"key=%{test_job.99999999999999.base_port+100}", false, ""},
		{"key=%{test_job.0.base_port+100}", true, "key=7101"},
		{"key=%{test_job.0.invalid_port+100}", false, ""},
		{"key=%{test_job.1.base_port+100}", true, "key=8101"},
		{"key=%{test_job.2.base_port+100}", false, ""},
		{"key=%{dependencies.0.test_job.server_list}", true, "key=www.example.com:7001,www.example.com:8001"},
		{"key=%{test_job.server_list}", true, "key=www.example.com:7001,www.example.com:8001"},
	}

	for i := range testCases {
		output, err := GlobalRender(c, testCases[i].input)
		if testCases[i].success {
			if err != nil {
				t.Fatalf("Test case #%d failed, %v", i, err)
			} else if output != testCases[i].output {
				t.Fatalf("Test case #%d failed, output mismatch, [%s]!=[%s]", i, output, testCases[i].output)
			}
		} else if err == nil {
			t.Fatalf("Test case #%d failed, shoud failed but success, output: %s", i, output)
		}
	}

	var testHostCases = []struct {
		input   string
		taskId  int
		success bool
		output  string
	}{
		{"key=%{test_job.x.host}", 0, true, "key=www.example.com"},
		{"key=%{invalid_job.x.host}", 0, false, ""},
		{"key=%{test_job.y.host}", 0, true, "key=%{test_job.y.host}"},
		{"key=%{test_job.x.invalid_host}", 0, false, ""},
		{"key=%{test_job.x.host}", 100, false, ""},
		{"key=%{test_job.x.base_port+100}", 0, true, "key=7101"},
		{"key=%{invalid_job.x.base_port+100}", 0, false, ""},
		{"key=%{invalid_job.x.base_port+999999999999}", 0, false, ""},
		{"key=%{test_job.y.base_port+100}", 0, true, "key=%{test_job.y.base_port+100}"},
		{"key=%{test_job.x.invalid_port+100}", 0, false, ""},
		{"key=%{test_job.x.base_port+200}", 1, true, "key=8201"},
		{"key=%{test_job.x.base_port+200}", 100, false, ""},
	}

	for i := range testHostCases {
		output, err := HostRender(c, testHostCases[i].taskId, testHostCases[i].input)
		if err != nil {
			if testHostCases[i].success {
				t.Fatalf("Test case #%d failed, should success but failed: %v", i, err)
			} else {
				continue
			}
		}
		if !testHostCases[i].success {
			t.Fatalf("Test case #%d failed, should failed but success", i)
		}
		if output != testHostCases[i].output {
			t.Fatalf("Test case #%d failed, output mismatch. [%s] != [%s]", i, output, testHostCases[i].output)
		}
	}
}
