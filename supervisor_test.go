package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	TEST_AGENT_PORT   = 9743
	TEST_PKG_SRV_PORT = 4321
)

type MiniHuker struct {
	supervisorSize int
	supervisor     []*Supervisor
	superClient    []*SupervisorCli
	pkgServer      *PackageServer
	wg             *sync.WaitGroup
}

func NewMiniHuker(supervisorSize int) *MiniHuker {
	agentRootDir := fmt.Sprintf("/tmp/huker/%d", time.Now().UnixNano())
	// mkdir root dir of agent if not exist.
	if _, err := os.Stat(agentRootDir); os.IsNotExist(err) {
		if err := os.MkdirAll(agentRootDir, 0755); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}
	var supervisors []*Supervisor
	var superClients []*SupervisorCli
	for i := 0; i < supervisorSize; i++ {
		supervisor, err := NewSupervisor(agentRootDir, TEST_AGENT_PORT+i, agentRootDir+"/supervisor.db"+strconv.Itoa(i))
		if err != nil {
			panic(err)
		}
		supervisors = append(supervisors, supervisor)

		superClient := &SupervisorCli{
			serverAddr: fmt.Sprintf("http://127.0.0.1:%d", TEST_AGENT_PORT+i),
		}
		superClients = append(superClients, superClient)
	}

	// Initialize the package server.
	p, err := NewPackageServer(TEST_PKG_SRV_PORT, "./testdata/lib", "./testdata/conf/pkg.yaml")
	if err != nil {
		panic(err)
	}

	m := &MiniHuker{
		supervisorSize: supervisorSize,
		supervisor:     supervisors,
		superClient:    superClients,
		pkgServer:      p,
		wg:             &sync.WaitGroup{},
	}

	m.wg.Add(m.supervisorSize + 1)
	return m
}

func (m *MiniHuker) Start() {
	// Start supervisor server
	for i := 0; i < m.supervisorSize; i++ {
		supervisor := m.supervisor[i]
		go func() {
			defer m.wg.Done()
			if err := supervisor.Start(); err != nil {
				log.Error(err)
			}
		}()
	}

	// Start package server
	go func() {
		defer m.wg.Done()
		if err := m.pkgServer.Start(); err != nil {
			log.Error(err)
		}
	}()

	// Wait until both supervisor and package server finished.
	time.Sleep(1 * time.Second)
}

func (m *MiniHuker) Stop() {
	for i := 0; i < m.supervisorSize; i++ {
		if err := m.supervisor[i].Shutdown(); err != nil {
			log.Error(err)
		}
	}
	if err := m.pkgServer.Shutdown(); err != nil {
		log.Error(err)
	}
	m.wg.Wait()
}

func NewProgram() *Program {
	return &Program{
		Name:   "tst-py",
		Job:    "http-server.4",
		TaskId: 100,
		Bin:    "python",
		Args:   []string{"-m", "SimpleHTTPServer", "8452"},
		Configs: map[string]string{
			"a": "b", "c": "d",
		},
		PkgAddress: fmt.Sprintf("http://127.0.0.1:%d/test.tar.gz", TEST_PKG_SRV_PORT),
		PkgName:    "test.tar.gz",
		PkgMD5Sum:  "f77f526dcfbdbfb2dd942b6628f4c0ab",
		Hooks:      make(map[string]string),
	}
}

func TestMiniHuker(t *testing.T) {
	m := NewMiniHuker(1)

	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.superClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	if p, err := m.superClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	} else if p.RootDir != path.Join(m.supervisor[0].rootDir, p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId)) {
		t.Fatalf("root directory of program mismatch. rootDir: %s", p.RootDir)
	}

	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.superClient[0].Restart(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("restart process failed: %v", err)
	}

	if p, err := m.superClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	}

	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.superClient[0].Cleanup(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("cleanup program failed: %v", err)
	}
}

func TestRollingUpdate(t *testing.T) {
	m := NewMiniHuker(1)

	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.superClient[0].Bootstrap(prog); err != nil {
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
		if err := m.superClient[0].RollingUpdate(prog); err != nil {
			t.Fatalf("RollingUpdate failed [config case]: %v", err)
		}
		if p, err := m.superClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
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
		if err := m.superClient[0].RollingUpdate(prog); err != nil {
			t.Fatalf("RollingUpdate failed [package case]: %v", err)
		}
		if p, err := m.superClient[0].Show(prog.Name, prog.Job, prog.TaskId); err != nil {
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

	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
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
	m := NewMiniHuker(1)
	m.Start()
	defer m.Stop()

	expected := fmt.Sprintf("%s\npython\n-m SimpleHTTPServer 8452\n%s/tst-py/http-server.4.100\ntst-py\nhttp-server.4\n100\n", m.supervisor[0].rootDir, m.supervisor[0].rootDir)

	prog := NewProgram()
	hookNames := []string{"pre_bootstrap", "post_bootstrap", "pre_start", "post_start",
		"pre_rolling_update", "post_rolling_update", "pre_stop", "post_stop", "pre_restart", "post_restart"}
	for _, hookName := range hookNames {
		prog.Hooks[hookName] = fmt.Sprintf(hookScript, hookName)
	}

	if err := m.superClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.superClient[0].Start(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.superClient[0].Restart(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
	// Config change for rolling update
	prog.Configs = map[string]string{
		"a": "b", "c": "d", "e": "f",
	}
	if err := m.superClient[0].RollingUpdate(prog); err != nil {
		t.Fatalf("%v", err)
	}
	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}

	for _, hookName := range hookNames {
		testHookFile := path.Join(m.supervisor[0].rootDir, HOOKS_DIR, prog.Name,
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
	m := NewMiniHuker(1)
	m.Start()
	defer m.Stop()

	prog := NewProgram()
	if err := m.superClient[0].Bootstrap(prog); err != nil {
		t.Fatalf("%v", err)
	}
	programs, err := m.superClient[0].ListTasks()
	if err != nil {
		t.Fatalf("List task failed: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("Size of programs should be 1")
	}
	newProg, err := m.superClient[0].GetTask(prog.Name, prog.Job, prog.TaskId)
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
	if newProg.Status != StatusRunning {
		t.Fatalf("Status mismatch, %s != %s", newProg.Status, StatusRunning)
	}
	if _, err := os.Stat(newProg.RootDir); err != nil {
		t.Fatalf("Stat RootDir failed, %s, err: %v", newProg.RootDir, err)
	}
	if !reflect.DeepEqual(newProg.Hooks, prog.Hooks) {
		t.Fatalf("Hooks mismatch, %v != %v", newProg.Hooks, prog.Hooks)
	}
	// Stop the job
	if err := m.superClient[0].Stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("%v", err)
	}
}
