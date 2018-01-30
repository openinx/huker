package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"os"
	"path"
)

// Constant key and value for the environment variables.
const (
	HUKER_CONF_DIR                = "HUKER_CONF_DIR"
	HUKER_CONF_DIR_DEFAULT        = "./conf"
	HUKER_PKG_HTTP_SERVER         = "HUKER_PKG_HTTP_SERVER"
	HUKER_PKG_HTTP_SERVER_DEFAULT = "http://127.0.0.1:4000"
)

type TaskResult struct {
	Host *Host
	Prog *Program
	Err  error
}

func NewTaskResult(host *Host, prog *Program, err error) TaskResult {
	return TaskResult{Host: host, Prog: prog, Err: err}
}

type HukerJob interface {
	Install(project, cluster, job string, taskId int) ([]TaskResult, error)
	Bootstrap(project, cluster, job string, taskId int) ([]TaskResult, error)
	Start(project, cluster, job string, taskId int) ([]TaskResult, error)
	Stop(project, cluster, job string, taskId int) ([]TaskResult, error)
	Restart(project, cluster, job string, taskId int) ([]TaskResult, error)
	RollingUpdate(project, cluster, job string, taskId int) ([]TaskResult, error)
	Show(project, cluster, job string, taskId int) ([]TaskResult, error)
	Cleanup(project, cluster, job string, taskId int) ([]TaskResult, error)
}

func NewDefaultHukerJob() (HukerJob, error) {
	configRootDir := ReadEnvStrValue(HUKER_CONF_DIR, HUKER_CONF_DIR_DEFAULT)
	pkgServerAddress := ReadEnvStrValue(HUKER_PKG_HTTP_SERVER, HUKER_PKG_HTTP_SERVER_DEFAULT)
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
		PkgRootDir:   path.Join(jobRootDir, PKG_DIR),
		PkgConfDir:   path.Join(jobRootDir, CONF_DIR),
		PkgDataDir:   path.Join(jobRootDir, DATA_DIR),
		PkgLogDir:    path.Join(jobRootDir, LOG_DIR),
		PkgStdoutDir: path.Join(jobRootDir, STDOUT_DIR),
	}

	c, err := LoadClusterConfig(clusterCfg, env)
	if err != nil {
		return nil, fmt.Errorf("Load service configuration failed, err: %v", err)
	}

	if _, ok := c.jobs[job]; !ok {
		return nil, fmt.Errorf("Job `%s` does not exist in %s", job, clusterCfg)
	}
	return c, nil
}

type updateFunc func(*Job, *Host, *supervisorCli, *Program) error

func (j *ConfigFileHukerJob) updateJob(project, cluster, job string, taskId int, update updateFunc) ([]TaskResult, error) {
	c, err := j.newCluster(project, cluster, job)
	if err != nil {
		return nil, err
	}

	jobPtr := c.jobs[job]
	var taskResults []TaskResult
	for _, host := range jobPtr.hosts {
		if taskId < 0 || taskId == host.taskId {
			cfgMap, err := c.RenderConfigFiles(jobPtr, host.taskId)
			if err != nil {
				log.Errorf("Failed to render config file, project: %s, cluster:%s, job:%s, taskId:%s",
					project, cluster, job, host.taskId)
				return nil, err
			}
			superClient := newSupervisorCli(host.toHttpAddress())
			prog := &Program{
				Name:       c.clusterName,
				Job:        job,
				TaskId:     host.taskId,
				Bin:        c.javaHome,
				Args:       jobPtr.toShell(),
				Configs:    cfgMap,
				PkgAddress: j.pkgServerAddress + "/" + c.packageName,
				PkgName:    c.packageName,
				PkgMD5Sum:  c.packageMd5sum,
				Hooks:      jobPtr.hooks,
			}
			taskResults = append(taskResults, NewTaskResult(host, nil, update(jobPtr, host, superClient, prog)))
		}
	}
	return taskResults, nil
}

func (j *ConfigFileHukerJob) Install(project, cluster, job string, taskId int) ([]TaskResult, error) {
	// TODO will implement this in #13
	return nil, nil
}

func (j *ConfigFileHukerJob) Bootstrap(project, cluster, job string, taskId int) ([]TaskResult, error) {
	return j.updateJob(project, cluster, job, taskId,
		func(jobPtr *Job, host *Host, s *supervisorCli, prog *Program) error {
			return s.bootstrap(prog)
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
		func(jobPtr *Job, host *Host, s *supervisorCli, prog *Program) error {
			return s.rollingUpdate(prog)
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
	jobPtr := c.jobs[job]
	var taskResults []TaskResult
	for _, host := range jobPtr.hosts {
		if taskId < 0 || taskId == host.taskId {
			supCli := newSupervisorCli(host.toHttpAddress())
			var err error
			if action == "Show" {
				prog, err := supCli.show(cluster, job, host.taskId)
				taskResults = append(taskResults, NewTaskResult(host, prog, err))
				continue
			}
			if action == "Start" {
				err = supCli.start(cluster, job, host.taskId)
			} else if action == "Stop" {
				err = supCli.stop(cluster, job, host.taskId)
			} else if action == "Restart" {
				err = supCli.restart(cluster, job, host.taskId)
			} else if action == "Cleanup" {
				err = supCli.cleanup(cluster, job, host.taskId)
			} else {
				return nil, fmt.Errorf("Unexpected action: %s", action)
			}
			taskResults = append(taskResults, NewTaskResult(host, nil, err))
		}
	}
	return taskResults, nil
}
