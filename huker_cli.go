package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
	"os"
	"os/exec"
	"path"
	"strings"
)

type HukerShell struct {
	cfgRootDir       string
	agentRootDir     string
	taskId           string
	pkgServerAddress string
}

func NewHukerShell(cfgRootDir, pkgServerAddress string) *HukerShell {
	return &HukerShell{
		cfgRootDir:       cfgRootDir,
		agentRootDir:     "$AgentRootDir", // it will render this variable by value at agent side.
		taskId:           "$TaskId",       // ditto
		pkgServerAddress: pkgServerAddress,
	}
}

func (h *HukerShell) Shell(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]

	p := &Program{
		Name:       args.cluster.clusterName,
		Job:        args.jobName,
		TaskId:     0, // default task id is 0 for local mode job.
		Bin:        args.cluster.javaHome,
		Args:       job.toShell(),
		Configs:    job.toConfigMap(),
		PkgAddress: fmt.Sprintf("%s/%s", h.pkgServerAddress, args.cluster.packageName),
		PkgName:    args.cluster.packageName,
		PkgMD5Sum:  args.cluster.packageMd5sum,
	}

	agentRootDir := c.String("dir")
	// TODO need to consider config change (or other host adjustment).
	if err := p.Install(agentRootDir); err != nil {
		if !strings.Contains(err.Error(), "already exists, cleanup it first please.") {
			return err
		}
	}

	// Start the command.
	cmd := exec.Command(p.Bin, p.Args...)
	cmd.Stderr, cmd.Stdout, cmd.Stdin = os.Stderr, os.Stdout, os.Stdin

	log.Infof("%s %s", p.Bin, strings.Join(p.Args, " "))

	return cmd.Run()
}

func (h *HukerShell) updateJobWithLatestConfig(c *cli.Context, updateFunc func(*Job, *Host, *SupervisorCli, *Program) error) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		cfgMap, err := args.cluster.RenderConfigFiles(job, host.taskId)
		if err != nil {
			return err
		}
		supCli := NewSupervisorCli(host.toHttpAddress())
		p := &Program{
			Name:       args.cluster.clusterName,
			Job:        args.jobName,
			TaskId:     host.taskId,
			Bin:        args.cluster.javaHome,
			Args:       job.toShell(),
			Configs:    cfgMap,
			PkgAddress: fmt.Sprintf("%s/%s", h.pkgServerAddress, args.cluster.packageName),
			PkgName:    args.cluster.packageName,
			PkgMD5Sum:  args.cluster.packageMd5sum,
			Hooks:      job.hooks,
		}
		updateFunc(job, host, supCli, p)
	}
	return nil
}

func (h *HukerShell) Bootstrap(c *cli.Context) error {
	return h.updateJobWithLatestConfig(c, func(job *Job, host *Host, cli *SupervisorCli, p *Program) error {
		if err := cli.bootstrap(p); err != nil {
			log.Errorf("Bootstrap job %s at %s failed, err: %v", job.jobName, host.toKey(), err)
		} else {
			log.Infof("Bootstrap job %s at %s -> Success", job.jobName, host.toKey())
		}
		return nil
	})
}

func (h *HukerShell) Show(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}
	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(host.toHttpAddress())
		if p, err := supCli.show(args.clusterName, args.jobName, host.taskId); err != nil {
			log.Errorf("Show job %s at %s failed, err: %v", args.jobName, host.toKey(), err)
		} else {
			log.Infof("Show job %s at %s is -> %s", args.jobName, host.toKey(), p.Status)
		}
	}
	return nil
}

func (h *HukerShell) Start(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(host.toHttpAddress())
		if err := supCli.start(args.clusterName, args.jobName, host.taskId); err != nil {
			log.Errorf("Start job %s at %s failed, err: %v", args.jobName, host.toKey(), err)
		} else {
			log.Infof("Start job %s at %s success.", args.jobName, host.toKey())
		}
	}
	return nil
}

func (h *HukerShell) Cleanup(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(host.toHttpAddress())
		if err := supCli.cleanup(args.clusterName, args.jobName, host.taskId); err != nil {
			log.Errorf("Cleanup job %s at %s failed, err: %v", args.jobName, host.toKey(), err)
		} else {
			log.Infof("Cleanup job %s at %s success.", args.jobName, host.toKey())
		}
	}
	return nil
}

func (h *HukerShell) RollingUpdate(c *cli.Context) error {
	return h.updateJobWithLatestConfig(c, func(job *Job, host *Host, cli *SupervisorCli, p *Program) error {
		if err := cli.rollingUpdate(p); err != nil {
			log.Errorf("RollingUpdate job %s at %s failed, err: %v", job.jobName, host.toKey(), err)
		} else {
			log.Infof("RollingUpdate job %s at %s -> Success", job.jobName, host.toKey())
		}
		return nil
	})
}

func (h *HukerShell) Restart(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(host.toHttpAddress())
		if err := supCli.restart(args.clusterName, args.jobName, host.taskId); err != nil {
			log.Errorf("Restart job %s at %s failed, err: %v", args.jobName, host.toKey(), err)
		} else {
			log.Infof("Restart job %s at %s success.", args.jobName, host.toKey())
		}
	}
	return nil
}

func (h *HukerShell) Stop(c *cli.Context) error {
	args, err := h.prevAction(c)
	if err != nil {
		return err
	}

	job := args.cluster.jobs[args.jobName]
	for _, host := range job.hosts {
		supCli := NewSupervisorCli(host.toHttpAddress())
		if err := supCli.stop(args.clusterName, args.jobName, host.taskId); err != nil {
			log.Errorf("Stop job %s at %s failed, err: %v", args.jobName, host.toKey(), err)
		} else {
			log.Infof("Stop job %s at %s success.", args.jobName, host.toKey())
		}
	}
	return nil
}

type PrevArgs struct {
	clusterName string
	project     string
	jobName     string
	cluster     *Cluster
	env         *EnvVariables
}

func (h *HukerShell) prevAction(c *cli.Context) (*PrevArgs, error) {
	project := c.String("project")     // TODO project field is required
	clusterName := c.String("cluster") // TODO cluster field is required
	jobName := c.String("job")

	projectPath := path.Join(h.cfgRootDir, project)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid project `%s`, create configuration under %s directory please.", project, projectPath)
	}

	clusterCfg := path.Join(projectPath, clusterName+".yaml")
	if _, err := os.Stat(clusterCfg); os.IsNotExist(err) {
		return nil, fmt.Errorf("Invalid cluster `%s`, %s does not exist.", clusterName, clusterCfg)
	}

	jobRootDir := path.Join(h.agentRootDir, clusterName, fmt.Sprintf("%s.%s", jobName, h.taskId))
	env := &EnvVariables{
		ConfRootDir:  h.cfgRootDir,
		PkgRootDir:   path.Join(jobRootDir, PKG_DIR),
		PkgConfDir:   path.Join(jobRootDir, CONF_DIR),
		PkgDataDir:   path.Join(jobRootDir, DATA_DIR),
		PkgLogDir:    path.Join(jobRootDir, LOG_DIR),
		PkgStdoutDir: path.Join(jobRootDir, STDOUT_DIR),
	}

	cluster, err := LoadClusterConfig(clusterCfg, env)
	if err != nil {
		return nil, fmt.Errorf("Load service configuration failed, err: %v", err)
	}

	if _, ok := cluster.jobs[jobName]; !ok {
		return nil, fmt.Errorf("Job `%s` does not exist in %s", jobName, clusterCfg)
	}

	cfg := &PrevArgs{
		clusterName: clusterName,
		project:     projectPath,
		jobName:     jobName,
		cluster:     cluster,
		env:         env,
	}

	return cfg, nil
}
