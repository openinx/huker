package minihuker

import (
	"fmt"
	"github.com/openinx/huker/pkg/supervisor"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"testing"
)

func NewProgram() *supervisor.Program {
	return &supervisor.Program{
		Name:   "tst-py",
		Job:    "http-server.4",
		TaskId: 100,
		Bin:    "python",
		Args:   []string{"-m", "SimpleHTTPServer", "8452"},
		Configs: map[string]string{
			"a": "b", "c": "d",
		},
		PkgAddress: fmt.Sprintf("http://127.0.0.1:%d/test.tar.gz", testPkgSrvPort),
		PkgName:    "test.tar.gz",
		PkgMD5Sum:  "f77f526dcfbdbfb2dd942b6628f4c0ab",
		Hooks:      make(map[string]string),
	}
}

func TestMiniHuker(t *testing.T) {
	m := NewTestingMiniHuker(1)

	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.SuperClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	if p, err := m.SuperClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != supervisor.StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	} else if p.RootDir != path.Join(m.Supervisor[0].RootDir(), p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId)) {
		t.Fatalf("root directory of program mismatch. rootDir: %s", p.RootDir)
	}

	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.SuperClient[0].Restart(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("restart process failed: %v", err)
	}

	if p, err := m.SuperClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != supervisor.StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	}

	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.SuperClient[0].Cleanup(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("cleanup program failed: %v", err)
	}
}

func TestRollingUpdate(t *testing.T) {
	m := NewTestingMiniHuker(1)

	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.SuperClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	// Update config files.
	configCases := []map[string]string{
		{
			"a": "b", "c": "d", "e": "f",
		},
		{
			"a": "bb", "c": "dd", "e": "f",
		},
	}
	for _, cas := range configCases {
		prog.Configs = cas
		if err := m.SuperClient[0].RollingUpdate(prog); err != nil {
			t.Fatalf("RollingUpdate failed [config case]: %v", err)
		}
		if p, err := m.SuperClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
			t.Fatalf("Show process failed: %v", err)
		} else if !reflect.DeepEqual(cas, p.Configs) {
			t.Errorf("Config files mismatch %v != %v", cas, p.Configs)
		}
	}

	// Update package
	pkgCases := [][]string{
		{"http://127.0.0.1:4321/test-2.6.6.tar.gz", "test-2.6.6.tar.gz", "ddb85c4ba8fe5c1d4ad8a216ae5cda6d"},
	}
	for _, cas := range pkgCases {
		prog.PkgAddress, prog.PkgName, prog.PkgMD5Sum = cas[0], cas[1], cas[2]
		if err := m.SuperClient[0].RollingUpdate(prog); err != nil {
			t.Fatalf("RollingUpdate failed [package case]: %v", err)
		}
		if p, err := m.SuperClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
			t.Fatalf("Show process failed: %v", err)
		} else if !reflect.DeepEqual(p.Configs, p.Configs) {
			t.Errorf("Config files mismatch %v != %v", p.Configs, prog.Configs)
		} else if p.PkgAddress != prog.PkgAddress {
			t.Errorf("PkgAddress mismatch %v != %v", p.PkgAddress, prog.PkgAddress)
		} else if p.PkgName != prog.PkgName {
			t.Errorf("PkgName mismatch %v != %v", p.PkgName, prog.PkgName)
		} else if p.PkgMD5Sum != prog.PkgMD5Sum {
			t.Errorf("PkgMD5Sum mismatch %v != %v", p.PkgMD5Sum, prog.PkgMD5Sum)
		}
	}

	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("Stop process failed: %v", err)
	}
}

const hookScript = `#!/bin/bash
file=$SUPERVISOR_ROOT_DIR/.hooks/$PROGRAM_NAME/$PROGRAM_JOB_NAME.$PROGRAM_TASK_ID/%s.sh
echo $SUPERVISOR_ROOT_DIR > $file
echo $PROGRAM_BIN >> $file
echo $PROGRAM_ARGS >> $file
echo $PROGRAM_DIR >> $file
echo $PROGRAM_NAME >> $file
echo $PROGRAM_JOB_NAME >> $file
echo $PROGRAM_TASK_ID >> $file
`

func TestHooks(t *testing.T) {
	m := NewTestingMiniHuker(1)
	m.Start()
	defer m.Stop()

	expected := fmt.Sprintf("%s\npython\n-m SimpleHTTPServer 8452\n%s/tst-py/http-server.4.100\ntst-py\nhttp-server.4\n100\n", m.Supervisor[0].RootDir(), m.Supervisor[0].RootDir())

	prog := NewProgram()
	hookNames := []string{"pre_bootstrap", "post_bootstrap", "pre_start", "post_start",
		"pre_rolling_update", "post_rolling_update", "pre_stop", "post_stop", "pre_restart", "post_restart"}
	for _, hookName := range hookNames {
		prog.Hooks[hookName] = fmt.Sprintf(hookScript, hookName)
	}

	if err := m.SuperClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.SuperClient[0].Start(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.SuperClient[0].Restart(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	// Config change for rolling update
	prog.Configs = map[string]string{
		"a": "b", "c": "d", "e": "f",
	}
	if err := m.SuperClient[0].RollingUpdate(prog); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}

	for _, hookName := range hookNames {
		testHookFile := path.Join(m.Supervisor[0].RootDir(), supervisor.HOOKS_DIR, prog.Name,
			fmt.Sprintf("%s.%d", prog.Job, prog.TaskId), hookName+".sh")
		data, err := ioutil.ReadFile(testHookFile)
		if err != nil {
			t.Fatalf("Failed to read %s, err: %v", testHookFile, err)
		}
		if expected != string(data) {
			t.Errorf("test hook file content mismatch: [%q] != [%q]", expected, string(data))
		}
	}
}

func TestListTasks(t *testing.T) {
	m := NewTestingMiniHuker(1)
	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.SuperClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("%v", err)
	}
	programs, err := m.SuperClient[0].ListTasks()
	if err != nil {
		t.Fatalf("List task failed: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("Size of programs should be 1")
	}
	newProg, err := m.SuperClient[0].GetTask(prog.Name, prog.Job, prog.TaskId)
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	if newProg.Name != prog.Name {
		t.Fatalf("Name mismatch, %s != %s", newProg.Name, prog.Name)
	}
	if newProg.Job != prog.Job {
		t.Fatalf("Job mismatch, %s != %s", newProg.Job, prog.Job)
	}
	if newProg.TaskId != prog.TaskId {
		t.Fatalf("TaskId mismatch, %d != %d", newProg.TaskId, prog.TaskId)
	}
	if newProg.Bin != prog.Bin {
		t.Fatalf("Bin mismatch, %s != %s", newProg.Bin, prog.Bin)
	}
	if !reflect.DeepEqual(newProg.Args, prog.Args) {
		t.Fatalf("Args mismatch, %v != %v", newProg.Args, prog.Args)
	}
	if !reflect.DeepEqual(newProg.Configs, prog.Configs) {
		t.Fatalf("Configs mismatch, %v != %v", newProg.Configs, prog.Configs)

	}
	if newProg.PkgAddress != prog.PkgAddress {
		t.Fatalf("PkgAddress mismatch, %s != %s", newProg.PkgAddress, prog.PkgAddress)
	}
	if newProg.PkgName != prog.PkgName {
		t.Fatalf("PkgName mismatch, %s != %s", newProg.PkgName, prog.PkgName)
	}
	if newProg.PkgMD5Sum != prog.PkgMD5Sum {
		t.Fatalf("PkgMD5Sum mismatch, %s != %s", newProg.PkgMD5Sum, prog.PkgMD5Sum)
	}
	if newProg.Status != supervisor.StatusRunning {
		t.Fatalf("Status mismatch, %s != %s", newProg.Status, supervisor.StatusRunning)
	}
	if _, err := os.Stat(newProg.RootDir); err != nil {
		t.Fatalf("Stat RootDir failed, %s, err: %v", newProg.RootDir, err)
	}
	if !reflect.DeepEqual(newProg.Hooks, prog.Hooks) {
		t.Fatalf("Hooks mismatch, %v != %v", newProg.Hooks, prog.Hooks)
	}
	// Stop the job
	if err := m.SuperClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
}
