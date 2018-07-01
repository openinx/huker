package minihuker

import (
	"fmt"
	dash "github.com/openinx/huker/pkg/dashboard"
	"github.com/openinx/huker/pkg/pkgsrv"
	"github.com/openinx/huker/pkg/supervisor"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	testAgentPort     = 9743
	testPkgSrvPort    = 4321
	testDashboardPort = 9008
)

type MiniHuker struct {
	SupervisorSize int
	Supervisor     []*supervisor.Supervisor
	SuperClient    []*supervisor.SupervisorCli
	PkgServer      *pkgsrv.PackageServer
	Dashboard      *dash.Dashboard
	WaitGroup      *sync.WaitGroup
}

func localHttpAddress(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func NewMiniHuker(cfgRootDir string, agentSize int, agentRootDir string, agentPort int,
	pkgSrvPort int, pkgSrvLibDir, pkgSrvConfFile string,
	dashboardPort int, grafanaAddress string) *MiniHuker {
	// Initialize the supervisor agents.
	if _, err := os.Stat(agentRootDir); os.IsNotExist(err) {
		if err := os.MkdirAll(agentRootDir, 0755); err != nil {
			panic(err)
		}
	}
	var supervisors []*supervisor.Supervisor
	var superClients []*supervisor.SupervisorCli
	for i := 0; i < agentSize; i++ {
		agent, err := supervisor.NewSupervisor(agentRootDir, agentPort+i, agentRootDir+"/supervisor.db"+strconv.Itoa(i))
		if err != nil {
			panic(err)
		}
		supervisors = append(supervisors, agent)

		superClient := &supervisor.SupervisorCli{
			ServerAddr: localHttpAddress(agentPort + i),
		}
		superClients = append(superClients, superClient)
	}

	// Initialize the package server.
	p, err := pkgsrv.NewPackageServer(pkgSrvPort, pkgSrvLibDir, pkgSrvConfFile)
	if err != nil {
		panic(err)
	}

	// Initialize the dashboard http server
	dashboard, err := dash.NewDashboard(dashboardPort, cfgRootDir, localHttpAddress(pkgSrvPort), grafanaAddress)
	if err != nil {
		panic(err)
	}

	m := &MiniHuker{
		SupervisorSize: agentSize,
		Supervisor:     supervisors,
		SuperClient:    superClients,
		PkgServer:      p,
		Dashboard:      dashboard,
		WaitGroup:      &sync.WaitGroup{},
	}

	// supervisors, a package server, a dashboard server.
	m.WaitGroup.Add(m.SupervisorSize + 1 + 1)
	return m
}

func NewTestingMiniHuker(supervisorSize int) *MiniHuker {
	agentRootDir := fmt.Sprintf("/tmp/huker/%d", time.Now().UnixNano())
	return NewMiniHuker(utils.GetHukerSourceDir()+"/testdata/conf",
		supervisorSize,
		agentRootDir,
		testAgentPort,
		testPkgSrvPort,
		utils.GetHukerSourceDir()+"/testdata/lib",
		utils.GetHukerSourceDir()+"/testdata/conf/pkg.yaml",
		testDashboardPort,
		localHttpAddress(3000))
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

	// Start the dashboard server
	go func() {
		defer m.WaitGroup.Done()
		if err := m.Dashboard.Start(); err != nil {
			log.Error(err)
		}
	}()

	// Wait until both supervisor and package server finished.
	time.Sleep(1 * time.Second)
}

func (m *MiniHuker) Wait() {
	m.WaitGroup.Wait()
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
	if err := m.Dashboard.Shutdown(); err != nil {
		log.Error(err)
	}
	m.WaitGroup.Wait()
}
