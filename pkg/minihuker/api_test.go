package minihuker

import (
	"github.com/openinx/huker/pkg/core"
	"github.com/openinx/huker/pkg/supervisor"
	"github.com/openinx/huker/pkg/utils"
	"testing"
)

func TestHukerJob(t *testing.T) {
	// TODO try to increase task size to 5. need to render(both global & host) the extra_args section.
	taskSize := 1

	miniHuker := NewTestingMiniHuker(taskSize)
	miniHuker.Start()
	defer miniHuker.Stop()

	hukerJob, err := core.NewConfigFileHukerJob(utils.GetHukerSourceDir()+"/testdata/conf", localHttpAddress(testPkgSrvPort))
	if err != nil {
		t.Fatal(err)
	}
	var results []core.TaskResult

	project, cluster, job := "pyserver", "py_test", "httpserver"
	// Test Bootstrap
	results, err = hukerJob.Bootstrap(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of bootstrap mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("Bootstrap task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		}
	}
	// Test Shell twice. initialize package the first time, skip when second time.
	for i := 0; i < 2; i++ {
		err = hukerJob.Shell(project, cluster, "shell", []string{})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Test Show
	results, err = hukerJob.Show(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of Show mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("Show task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		} else if results[i].Prog.Status != supervisor.StatusRunning {
			t.Errorf("Status of %s shoud be Running. other than %s", results[i].Host.ToKey(), results[i].Prog.Status)
		}
	}
	// Test Start
	results, err = hukerJob.Start(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of Start mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err == nil {
			t.Errorf("Start task %s shoud be failed", results[i].Host.ToKey())
		}
	}
	// Test Restart
	results, err = hukerJob.Restart(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of Restart mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("Restart task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		}
	}
	// Test RollingUpdate
	results, err = hukerJob.RollingUpdate(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of RollingUpdate mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("RollingUpdate task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		}
	}
	// Test Stop
	results, err = hukerJob.Stop(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of Stop mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("Stop task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		}
	}
	// Test Cleanup
	results, err = hukerJob.Cleanup(project, cluster, job, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != taskSize {
		t.Fatalf("Result size of Cleanup mismatch, %d != %d", len(results), taskSize)
	}
	for i := 0; i < taskSize; i++ {
		if results[i].Err != nil {
			t.Errorf("Cleanup task %s failed, %v", results[i].Host.ToKey(), results[i].Err)
		}
	}
}

func TestHukerJobList(t *testing.T) {
	hukerJob, err := core.NewConfigFileHukerJob(utils.GetHukerSourceDir()+"/testdata/conf", localHttpAddress(testPkgSrvPort))
	if err != nil {
		t.Fatal(err)
	}
	clusters, errs := hukerJob.List()
	if errs != nil {
		t.Fatal(errs)
	}
	if len(clusters) != 1 {
		t.Fatalf("Cluster size should be %d", 1)
	}
	c := clusters[0]
	if c.ClusterName != "py_test" {
		t.Fatalf("cluster name mismatch, %s != %s", c.ClusterName, "py_test")
	}
	if len(c.Jobs) != 2 {
		t.Fatalf("Jobs of cluster should be %d, instead of %d", 2, len(c.Jobs))
	}
	for key := range c.Jobs {
		if key != "httpserver" && key != "shell" {
			t.Fatalf("Job name of cluster shoud be %s or %s", "httpserver", "shell")
		}
	}
}
