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
			Job:        args.jobName,
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
			log.Infof("Bootstrap job %s at %s success.", args.jobName, host)
		}
	}
	return nil
}

func (h *HukerShell) Show(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))
		if p, err := supCli.show(args.cluster, args.jobName); err != nil {
			log.Errorf("Show job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Show job %s at %s is -> %s", args.jobName, host, p.Status)
		}
	}
	return nil
}

func (h *HukerShell) Start(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))
		if err := supCli.start(args.cluster, args.jobName); err != nil {
			log.Errorf("Start job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Start job %s at %s success.", args.jobName, host)
		}
	}
	return nil
}

func (h *HukerShell) Cleanup(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))
		if err := supCli.cleanup(args.cluster, args.jobName); err != nil {
			log.Errorf("Cleanup job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Cleanup job %s at %s success.", args.jobName, host)
		}
	}
	return nil
}

func (h *HukerShell) RollingUpdate(c *cli.Context) error {
	return nil
}

func (h *HukerShell) Restart(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))
		if err := supCli.restart(args.cluster, args.jobName); err != nil {
			log.Errorf("Restart job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Restart job %s at %s success.", args.jobName, host)
		}
	}
	return nil
}

func (h *HukerShell) Stop(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		log.Error(err)
		return err
	}

	job := args.srvCfg.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(fmt.Sprintf("http://%s:%d", host, h.supervisorPort))
		if err := supCli.stop(args.cluster, args.jobName); err != nil {
			log.Errorf("Stop job %s at %s failed, err: %v", args.jobName, host, err)
		} else {
			log.Infof("Stop job %s at %s success.", args.jobName, host)
		}
	}
	return nil
}

type PrevArgs struct {
	cluster string
	project string
	jobName string
	srvCfg  *ServiceConfig
	env     *EnvVariables
}

func (h *HukerShell) prevAction(c *cli.Context) (*PrevArgs, error) {
	project := c.String("project")
	cluster := c.String("cluster")
	jobName := c.String("job")

	projectPath := path.Join(h.cfgRootDir, project)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid service `%s`, create configuration under %s directory please.", project, projectPath)
	}

	clusterCfg := path.Join(projectPath, cluster+".yaml")
	if _, err := os.Stat(clusterCfg); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid cluster `%s`, %s does not exist.", cluster, clusterCfg)
	}

	env := &EnvVariables{
		ConfRootDir:  h.cfgRootDir,
		PkgRootDir:   path.Join(h.agentRootDir, cluster, jobName, PKG_DIR),
		PkgConfDir:   path.Join(h.agentRootDir, cluster, jobName, CONF_DIR),
		PkgDataDir:   path.Join(h.agentRootDir, cluster, jobName, DATA_DIR),
		PkgLogDir:    path.Join(h.agentRootDir, cluster, jobName, LOG_DIR),
		PkgStdoutDir: path.Join(h.agentRootDir, cluster, jobName, STDOUT_DIR),
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
		project: projectPath,
		jobName: jobName,
		srvCfg:  srvCfg,
		env:     env,
	}

	return cfg, nil
}
