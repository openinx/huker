package core

import (
	"fmt"
	"github.com/openinx/huker/pkg/supervisor"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
)

// Constant key and value for the environment variables.
const (
	HUKER_CONF_DIR                = "HUKER_CONF_DIR"
	HUKER_CONF_DIR_DEFAULT        = "./conf"
	HUKER_PKG_HTTP_SERVER         = "HUKER_PKG_HTTP_SERVER"
	HUKER_PKG_HTTP_SERVER_DEFAULT = "http://127.0.0.1:4000"
	defaultLocalTaskId            = 0
)

type TaskResult struct {
	Host *Host
	Prog *supervisor.Program
	Err  error
}

func NewTaskResult(host *Host, prog *supervisor.Program, err error) TaskResult {
	return TaskResult{Host: host, Prog: prog, Err: err}
}

type HukerJob interface {
	List() ([]*Cluster, error)
	Install(project, cluster, job string, taskId int) ([]TaskResult, error)
	Shell(project, cluster, job string, extraArgs []string) error
	Bootstrap(project, cluster, job string, taskId int) ([]TaskResult, error)
	Start(project, cluster, job string, taskId int) ([]TaskResult, error)
	Stop(project, cluster, job string, taskId int) ([]TaskResult, error)
	Restart(project, cluster, job string, taskId int) ([]TaskResult, error)
	RollingUpdate(project, cluster, job string, taskId int) ([]TaskResult, error)
	Show(project, cluster, job string, taskId int) ([]TaskResult, error)
	Cleanup(project, cluster, job string, taskId int) ([]TaskResult, error)
}

func NewDefaultHukerJob() (HukerJob, error) {
	configRootDir := utils.ReadEnvStrValue(HUKER_CONF_DIR, HUKER_CONF_DIR_DEFAULT)
	pkgServerAddress := utils.ReadEnvStrValue(HUKER_PKG_HTTP_SERVER, HUKER_PKG_HTTP_SERVER_DEFAULT)
	return NewConfigFileHukerJob(configRootDir, pkgServerAddress)
}

type ConfigFileHukerJob struct {
	configRootDir    string
	pkgServerAddress string
}

func NewConfigFileHukerJob(configRootDir, pkgServerAddress string) (*ConfigFileHukerJob, error) {
	if _, err := os.Stat(configRootDir); err != nil {
		return nil, err
	}
	return &ConfigFileHukerJob{
		configRootDir:    configRootDir,
		pkgServerAddress: pkgServerAddress,
	}, nil
}

func (cfg *ConfigFileHukerJob) newCluster(project, cluster, job string) (*Cluster, error) {
	projectPath := path.Join(cfg.configRootDir, project)
	if _, err := os.Stat(projectPath); err != nil {
		return nil, fmt.Errorf("Invalid project `%s`, create configuration under %s directory please.", project, projectPath)
	}

	clusterCfg := path.Join(projectPath, cluster+".yaml")
	if _, err := os.Stat(clusterCfg); err != nil {
		return nil, fmt.Errorf("Invalid cluster `%s`, %s does not exist.", cluster, clusterCfg)
	}

	jobRootDir := path.Join("$AgentRootDir", cluster, fmt.Sprintf("%s.$TaskId", job))
	env := &EnvVariables{
		ConfRootDir:  cfg.configRootDir,
		PkgRootDir:   path.Join(jobRootDir, supervisor.PKG_DIR),
		PkgConfDir:   path.Join(jobRootDir, supervisor.CONF_DIR),
		PkgDataDir:   path.Join(jobRootDir, supervisor.DATA_DIR),
		PkgLogDir:    path.Join(jobRootDir, supervisor.LOG_DIR),
		PkgStdoutDir: path.Join(jobRootDir, supervisor.STDOUT_DIR),
	}

	c, err := LoadClusterConfig(clusterCfg, env)
	if err != nil {
		return nil, fmt.Errorf("Load service configuration failed, err: %v", err)
	}

	if _, ok := c.Jobs[job]; !ok {
		return nil, fmt.Errorf("Job `%s` does not exist in %s", job, clusterCfg)
	}
	return c, nil
}

type updateFunc func(*Job, *Host, *supervisor.SupervisorCli, *supervisor.Program) error

func (j *ConfigFileHukerJob) updateJob(project, cluster, job string, taskId int, update updateFunc) ([]TaskResult, error) {
	c, err := j.newCluster(project, cluster, job)
	if err != nil {
		return nil, err
	}

	jobPtr := c.Jobs[job]
	var taskResults []TaskResult
	for _, host := range jobPtr.Hosts {
		if taskId < 0 || taskId == host.TaskId {
			cfgMap, err := c.RenderConfigFiles(jobPtr, host.TaskId, false)
			if err != nil {
				log.Errorf("Failed to render config file, project: %s, cluster:%s, job:%s, taskId:%d",
					project, cluster, job, host.TaskId)
				return nil, err
			}
			superClient := supervisor.NewSupervisorCli(host.ToHttpAddress())
			prog := &supervisor.Program{
				Name:       c.ClusterName,
				Job:        job,
				TaskId:     host.TaskId,
				Bin:        c.MainProcess,
				Args:       jobPtr.toShell(),
				Configs:    cfgMap,
				PkgAddress: j.pkgServerAddress + "/" + c.PackageName,
				PkgName:    c.PackageName,
				PkgMD5Sum:  c.PackageMd5sum,
				Hooks:      jobPtr.Hooks,
			}
			taskResults = append(taskResults, NewTaskResult(host, nil, update(jobPtr, host, superClient, prog)))
		}
	}
	return taskResults, nil
}

func (j *ConfigFileHukerJob) List() ([]*Cluster, error) {
	files, err := ioutil.ReadDir(j.configRootDir)
	if err != nil {
		return nil, err
	}
	var clusters []*Cluster
	for _, f := range files {
		if f.IsDir() {
			subFiles, err := ioutil.ReadDir(path.Join(j.configRootDir, f.Name()))
			if err != nil {
				return nil, err
			}
			for _, subFile := range subFiles {
				if !subFile.IsDir() {
					c, err := LoadClusterConfig(path.Join(j.configRootDir, f.Name(), subFile.Name()),
						&EnvVariables{
							ConfRootDir:  j.configRootDir,
							PkgRootDir:   "unknown",
							PkgConfDir:   "unknown",
							PkgDataDir:   "unknown",
							PkgLogDir:    "unknown",
							PkgStdoutDir: "unknown",
						})
					if err != nil {
						return nil, err
					}
					clusters = append(clusters, c)
				}
			}
		}
	}
	return clusters, nil
}

func (j *ConfigFileHukerJob) Install(project, cluster, job string, taskId int) ([]TaskResult, error) {
	// TODO will implement this in #13
	return nil, nil
}

func (j *ConfigFileHukerJob) Shell(project, cluster, job string, extraArgs []string) error {
	c, err := j.newCluster(project, cluster, job)
	if err != nil {
		return err
	}
	jobPtr := c.Jobs[job]
	cfgMap, err := c.RenderConfigFiles(jobPtr, defaultLocalTaskId, true)
	if err != nil {
		log.Errorf("Failed to render config file, project: %s, cluster:%s, job:%s, taskId:%d",
			project, cluster, job, defaultLocalTaskId)
		return err
	}
	prog := &supervisor.Program{
		Name:       c.ClusterName,
		Job:        job,
		TaskId:     defaultLocalTaskId,
		Bin:        c.MainProcess,
		Args:       jobPtr.toShell(),
		Configs:    cfgMap,
		PkgAddress: j.pkgServerAddress + "/" + c.PackageName,
		PkgName:    c.PackageName,
		PkgMD5Sum:  c.PackageMd5sum,
		Hooks:      jobPtr.Hooks,
	}
	agentRootDir := utils.LocalHukerDir()
	prog.RenderVars(agentRootDir)
	if err := prog.Install(agentRootDir); err != nil {
		if !strings.Contains(err.Error(), "already exists, cleanup it first please.") {
			return err
		} else {
			if err := prog.UpdatePackage(agentRootDir); err != nil {
				return err
			}
			if err := prog.DumpConfigFiles(agentRootDir); err != nil {
				return err
			}
		}
	}
	// Start the command.
	args := append(prog.Args, extraArgs...)
	cmd := exec.Command(prog.Bin, args...)
	cmd.Stderr, cmd.Stdout, cmd.Stdin = os.Stderr, os.Stdout, os.Stdin
	log.Debugf("%s %s", prog.Bin, strings.Join(args, " "))
	return cmd.Run()
}

func (j *ConfigFileHukerJob) Bootstrap(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.updateJob(project, cluster, job, taskId,
		func(jobPtr *Job, host *Host, s *supervisor.SupervisorCli, prog *supervisor.Program) error {
			return s.Bootstrap(prog)
		})
}

func (j *ConfigFileHukerJob) Show(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.lookupJob(project, cluster, job, taskId, "Show")
}

func (j *ConfigFileHukerJob) Start(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.lookupJob(project, cluster, job, taskId, "Start")
}

func (j *ConfigFileHukerJob) Stop(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.lookupJob(project, cluster, job, taskId, "Stop")
}

func (j *ConfigFileHukerJob) Restart(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.lookupJob(project, cluster, job, taskId, "Restart")
}

func (j *ConfigFileHukerJob) RollingUpdate(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.updateJob(project, cluster, job, taskId,
		func(jobPtr *Job, host *Host, s *supervisor.SupervisorCli, prog *supervisor.Program) error {
			return s.RollingUpdate(prog)
		})
}

func (j *ConfigFileHukerJob) Cleanup(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.lookupJob(project, cluster, job, taskId, "Cleanup")
}

func (j *ConfigFileHukerJob) lookupJob(project, cluster, job string, taskId int, action string) ([]TaskResult, error) {
	c, err := j.newCluster(project, cluster, job)
	if err != nil {
		return nil, err
	}
	jobPtr := c.Jobs[job]
	var taskResults []TaskResult
	for _, host := range jobPtr.Hosts {
		if taskId < 0 || taskId == host.TaskId {
			supCli := supervisor.NewSupervisorCli(host.ToHttpAddress())
			var err error
			if action == "Show" {
				prog, err := supCli.Show(cluster, job, host.TaskId)
				taskResults = append(taskResults, NewTaskResult(host, prog, err))
				continue
			}
			if action == "Start" {
				err = supCli.Start(cluster, job, host.TaskId)
			} else if action == "Stop" {
				err = supCli.Stop(cluster, job, host.TaskId)
			} else if action == "Restart" {
				err = supCli.Restart(cluster, job, host.TaskId)
			} else if action == "Cleanup" {
				err = supCli.Cleanup(cluster, job, host.TaskId)
			} else {
				return nil, fmt.Errorf("Unexpected action: %s", action)
			}
			taskResults = append(taskResults, NewTaskResult(host, nil, err))
		}
	}
	return taskResults, nil
}
