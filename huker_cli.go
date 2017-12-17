package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
	"os"
	"path"
)

type HukerShell struct {
	cfgRootDir       string
	agentRootDir     string
	pkgServerAddress string
	supervisorPort   int
}

func NewHukerShell(cfgRootDir, agentRootDir, pkgServerAddress string, supervisorPort int) (*HukerShell, error) {
	h := &HukerShell{
		cfgRootDir:       cfgRootDir,
		agentRootDir:     agentRootDir,
		pkgServerAddress: pkgServerAddress,
		supervisorPort:   supervisorPort,
	}

	if _, err := os.Stat(cfgRootDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("Root configuration directory %s does not exist.", cfgRootDir)
	}

	return h, nil
}

func (h *HukerShell) Bootstrap(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))

		p := &Program{
			Name:       args.srvCfg.clusterName,
			Bin:        args.srvCfg.javaHome,
			Args:       job.toShell(),
			Configs:    job.toConfigMap(),
			PkgAddress: fmt.Sprintf("%s/%s", h.pkgServerAddress, args.srvCfg.packageName),
			PkgName:    args.srvCfg.packageName,
			PkgMD5Sum:  args.srvCfg.packageMd5sum,
		}
		if err := supCli.bootstrap(p); err != nil {
			log.Errorf("Bootstrap job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Bootstrap job %s at %s successfully.", args.jobName, host)
		}
	}
	return nil
}

type PrevArgs struct {
	cluster string
	service string
	jobName string
	srvCfg  *ServiceConfig
	env     *EnvVariables
}

func (h *HukerShell) prevAction(c *cli.Context) (*PrevArgs, error) {
	cluster := c.String("cluster")
	service := c.String("service")
	jobName := c.String("job")

	servicePath := path.Join(h.cfgRootDir, service)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid service `%s`, create configuration under %s directory please.", service, servicePath)
	}

	clusterCfg := path.Join(servicePath, cluster+".yaml")
	if _, err := os.Stat(clusterCfg); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid cluster `%s`, %s does not exist.", cluster, clusterCfg)
	}

	env := &EnvVariables{
		ConfRootDir:  h.cfgRootDir,
		PkgRootDir:   path.Join(h.agentRootDir, cluster, PKG_DIR),
		PkgConfDir:   path.Join(h.agentRootDir, cluster, CONF_DIR),
		PkgDataDir:   path.Join(h.agentRootDir, cluster, DATA_DIR),
		PkgLogDir:    path.Join(h.agentRootDir, cluster, LOG_DIR),
		PkgStdoutDir: path.Join(h.agentRootDir, cluster, STDOUT_DIR),
	}

	srvCfg, err := LoadServiceConfig(clusterCfg, env)
	if err != nil {
		return nil, fmt.Errorf("Load service configuration failed, err: %v", err)
	}

	if _, ok := srvCfg.jobs[jobName]; !ok {
		return nil, fmt.Errorf("Job `%s` does not exist in %s", jobName, clusterCfg)
	}

	cfg := &PrevArgs{
		cluster: cluster,
		service: service,
		jobName: jobName,
		srvCfg:  srvCfg,
		env:     env,
	}

	return cfg, nil
}
