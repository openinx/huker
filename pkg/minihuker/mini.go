package minihuker

import (
	"fmt"
	"github.com/openinx/huker/pkg/pkgsrv"
	"github.com/openinx/huker/pkg/supervisor"
	"github.com/qiniu/log"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
	"time"
)

const (
	TEST_AGENT_PORT   = 9743
	TEST_PKG_SRV_PORT = 4321
)

type MiniHuker struct {
	SupervisorSize int
	Supervisor     []*supervisor.Supervisor
	SuperClient    []*supervisor.SupervisorCli
	PkgServer      *pkgsrv.PackageServer
	WaitGroup      *sync.WaitGroup
}

func GetTestDataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information")
	}
	return path.Dir(path.Dir(path.Dir(filename)))
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
	var supervisors []*supervisor.Supervisor
	var superClients []*supervisor.SupervisorCli
	for i := 0; i < supervisorSize; i++ {
		agent, err := supervisor.NewSupervisor(agentRootDir, TEST_AGENT_PORT+i, agentRootDir+"/supervisor.db"+strconv.Itoa(i))
		if err != nil {
			panic(err)
		}
		supervisors = append(supervisors, agent)

		superClient := &supervisor.SupervisorCli{
			ServerAddr: fmt.Sprintf("http://127.0.0.1:%d", TEST_AGENT_PORT+i),
		}
		superClients = append(superClients, superClient)
	}

	// Initialize the package server.
	p, err := pkgsrv.NewPackageServer(TEST_PKG_SRV_PORT, GetTestDataDir()+"/testdata/lib", GetTestDataDir()+"/testdata/conf/pkg.yaml")
	if err != nil {
		panic(err)
	}

	m := &MiniHuker{
		SupervisorSize: supervisorSize,
		Supervisor:     supervisors,
		SuperClient:    superClients,
		PkgServer:      p,
		WaitGroup:      &sync.WaitGroup{},
	}

	m.WaitGroup.Add(m.SupervisorSize + 1)
	return m
}

func (m *MiniHuker) Start() {
	// Start supervisor server
	for i := 0; i < m.SupervisorSize; i++ {
		supervisor := m.Supervisor[i]
		go func() {
			defer m.WaitGroup.Done()
			if err := supervisor.Start(); err != nil {
				log.Error(err)
			}
		}()
	}

	// Start package server
	go func() {
		defer m.WaitGroup.Done()
		if err := m.PkgServer.Start(); err != nil {
			log.Error(err)
		}
	}()

	// Wait until both supervisor and package server finished.
	time.Sleep(1 * time.Second)
}

func (m *MiniHuker) Stop() {
	for i := 0; i < m.SupervisorSize; i++ {
		if err := m.Supervisor[i].Shutdown(); err != nil {
			log.Error(err)
		}
	}
	if err := m.PkgServer.Shutdown(); err != nil {
		log.Error(err)
	}
	m.WaitGroup.Wait()
}
