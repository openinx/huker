package huker

import (
	"fmt"
	"os"
	"testing"
)

func TestHukerJob(t *testing.T) {
	// TODO try to increase task size to 5. need to render(both global & host) the extra_args section.
	taskSize := 1
	miniHuker := NewMiniHuker(taskSize)
	miniHuker.Start()
	defer miniHuker.Stop()

	os.Setenv(HUKER_CONF_DIR, "./testdata/conf")
	os.Setenv(HUKER_PKG_HTTP_SERVER, fmt.Sprintf("http://127.0.0.1:%d", miniHuker.pkgServer.port))

	hukerJob, err := NewDefaultHukerJob()
	if err != nil {
		t.Fatal(err)
	}
	var results []TaskResult

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
		} else if results[i].Prog.Status != StatusRunning {
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
