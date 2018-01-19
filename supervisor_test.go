package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"os"
	"path"
	"testing"
	"time"
)

type MiniHuker struct {
	s   *Supervisor
	cli *SupervisorCli
	p   *PackageServer
}

func NewMiniHuker(agentRootDir string) *MiniHuker {
	// mkdir root dir of agent if not exist.
	if _, err := os.Stat(agentRootDir); os.IsNotExist(err) {
		os.MkdirAll(agentRootDir, 755)
	}
	supervisor, err := NewSupervisor(agentRootDir, 9743, agentRootDir+"/supervisor.db")
	if err != nil {
		panic(err)
	}
	m := &MiniHuker{
		s: supervisor,
		cli: &SupervisorCli{
			serverAddr: "http://127.0.0.1:9743",
		},
	}

	// Initialize the package server.
	p, err := NewPackageServer(4321, "./testdata/lib", "./testdata/conf/pkg.yaml")
	if err != nil {
		panic(err)
	}
	m.p = p

	return m
}

func (m *MiniHuker) start() {
	// Start supervisor server
	go func() {
		if err := m.s.Start(); err != nil {
			log.Error(err)
		}
	}()

	// Start package server
	go func() {
		if err := m.p.Start(); err != nil {
			log.Error(err)
		}
	}()
}

func TestMiniHuker(t *testing.T) {

	agentRootDir := fmt.Sprintf("/tmp/huker/%d", int32(time.Now().Unix()))
	m := NewMiniHuker(agentRootDir)

	// Wait supervisor server and package server start finished.
	m.start()
	time.Sleep(1 * time.Second)

	prog := &Program{
		Name:   "tst-py",
		Job:    "http-server.4",
		TaskId: 100,
		Bin:    "python",
		Args:   []string{"-m", "SimpleHTTPServer"},
		Configs: map[string]string{
			"a": "b", "c": "d",
		},
		PkgAddress: "http://127.0.0.1:4321/test.tar.gz",
		PkgName:    "test.tar.gz",
		PkgMD5Sum:  "f77f526dcfbdbfb2dd942b6628f4c0ab",
	}

	if err := m.cli.bootstrap(prog); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	if p, err := m.cli.show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	} else if p.RootDir != path.Join(agentRootDir, p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId)) {
		t.Fatalf("root directory of program mismatch. rootDir: %s", p.RootDir)
	}

	if err := m.cli.stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.cli.restart(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("restart process failed: %v", err)
	}

	if p, err := m.cli.show(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("show process failed: %v", err)
	} else if p.Status != StatusRunning {
		t.Fatalf("process is not running, cause: %v", err)
	}

	if err := m.cli.stop(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("stop process failed: %v", err)
	}

	if err := m.cli.cleanup(prog.Name, prog.Job, prog.TaskId); err != nil {
		t.Fatalf("cleanup program faile: %v", err)
	}
}
