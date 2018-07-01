package thirdparts

import (
	"errors"
	"testing"
)

func TestParseClusterJobTask(t *testing.T) {
	var testCases = []struct {
		label   string
		cluster string
		job     string
		task    int
		err     error
	}{
		{"cluster=test-hbase/job=master/task_id=0", "test-hbase", "master", 0, nil},
		{"job=master/task_id=0", "", "", 0, errors.New("Splits should be 3, lable:job=master/task_id=0")},
		{"c=test-hbase/master/task_id=0", "", "", 0, errors.New("cluster not found, label:c=test-hbase/master/task_id=0")},
		{"cluster=test-hbase/master/task=0", "", "", 0, errors.New("job not found, label:cluster=test-hbase/master/task=0")},
		{"cluster=test-hbase/job=master/task=0", "", "", 0, errors.New("task_id not found, label:cluster=test-hbase/job=master/task=0")},
	}
	for i, tc := range testCases {
		cluster, job, task, err := parseClusterJobTask(tc.label)
		if tc.cluster != cluster {
			t.Errorf("Case#%d cluster mismatch, %s != %s", i, tc.cluster, cluster)
		}
		if tc.job != job {
			t.Errorf("Case#%d job mismatch, %s != %s", i, tc.job, job)
		}
		if tc.task != task {
			t.Errorf("Case#%d task mismatch, %d != %d", i, tc.task, task)
		}
		if tc.err == nil || err == nil {
			if tc.err != nil || err != nil {
				t.Errorf("Case#%d all errors should be nil, [%v] != [%v]", i, tc.err, err)
			}
		} else if tc.err.Error() != err.Error() {
			t.Errorf("Case#%d error mismatch, [%v] != [%v]", i, tc.err, err)
		}
	}
}
