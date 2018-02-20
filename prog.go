package huker

import (
	"fmt"
	"github.com/qiniu/log"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Directories and status for supervisor agent.
const (
	DATA_DIR           = "data"
	LOG_DIR            = "log"
	PKG_DIR            = "pkg"
	CONF_DIR           = "conf"
	LIBRARY_DIR        = ".packages"
	STDOUT_DIR         = "stdout"
	HOOKS_DIR          = ".hooks"
	StatusRunning      = "Running"
	StatusStopped      = "Stopped"
	StatusNotBootstrap = "NotBootstrap"
	StatusUnknown      = "Unknown"
)

func progDirs() []string {
	return []string{DATA_DIR, LOG_DIR, CONF_DIR, STDOUT_DIR}
}

// Program is the process entry to manager in supervisor agent. one agent can manage multiple programs.
type Program struct {
	Name       string            `json:"name"`
	Job        string            `json:"job"`
	TaskId     int               `json:"task_id"`
	Bin        string            `json:"bin"`
	Args       []string          `json:"args"`
	Configs    map[string]string `json:"configs"`
	PkgAddress string            `json:"pkg_address"`
	PkgName    string            `json:"pkg_name"`
	PkgMD5Sum  string            `json:"pkg_md5sum"`
	PID        int               `json:"pid"`
	Status     string            `json:"status"`
	RootDir    string            `json:"root_dir"`
	Hooks      map[string]string `json:"hooks"`
}

// <agent-root-dir>/<cluster-name>/<job-name>.<task-id>
func (p *Program) getJobRootDir(agentRootDir string) string {
	return path.Join(agentRootDir, p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId))
}

// Update <job-root-dir>/pkg link.
func (p *Program) updatePackage(agentRootDir string) error {
	libsDir := path.Join(agentRootDir, LIBRARY_DIR)
	tmpPackageDir := path.Join(libsDir, fmt.Sprintf("%s.tmp", p.PkgMD5Sum))
	md5sumPackageDir := path.Join(libsDir, p.PkgMD5Sum)
	// Step.0 Create <agent-root-dir>/.packages/<md5sum>.tmp directory if not exists.
	if _, err := os.Stat(md5sumPackageDir); os.IsNotExist(err) {
		if _, err := os.Stat(tmpPackageDir); err == nil {
			if err := os.RemoveAll(tmpPackageDir); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(tmpPackageDir, 0755); err != nil {
			return err
		}

		// step.1 Download the package
		pkgFilePath := path.Join(tmpPackageDir, p.PkgName)
		if err := WebGetToLocal(p.PkgAddress, pkgFilePath); err != nil {
			return err
		}

		// step.2 Verify md5 checksum
		md5sum, md5Err := calcFileMD5Sum(pkgFilePath)
		if md5Err != nil {
			log.Errorf("Calculate the md5 checksum of file %s failed, cause: %v", pkgFilePath, md5Err)
			return md5Err
		}
		if md5sum != p.PkgMD5Sum {
			return fmt.Errorf("md5sum mismatch, %s != %s, package: %s", md5sum, p.PkgMD5Sum, p.PkgName)
		}

		// step.3 Extract package
		if err := RunCommand("tar", nil, "xzvf", pkgFilePath, "-C", tmpPackageDir); err != nil {
			return err
		}
		if err := os.Rename(tmpPackageDir, md5sumPackageDir); err != nil {
			return err
		}
	}

	// Step.4 Link <job-root-dir>/pkg to <agent-root-dir>/packages/<md5sum>
	files, errs := ioutil.ReadDir(md5sumPackageDir)
	if errs != nil {
		return errs
	}
	for _, f := range files {
		if f.IsDir() {
			linkPkgDir, err := filepath.Abs(path.Join(p.getJobRootDir(agentRootDir), PKG_DIR))
			if err != nil {
				return err
			}
			pkgDir := path.Join(md5sumPackageDir, f.Name())
			if _, err := os.Stat(linkPkgDir); err == nil {
				os.RemoveAll(linkPkgDir)
			}
			return os.Symlink(pkgDir, linkPkgDir)
		}
	}
	return fmt.Errorf("Sub-directory under %s does not exist.", md5sumPackageDir)
}

func (p *Program) dumpConfigFiles(agentRootDir string) error {
	for fname, content := range p.Configs {
		// When fname is /tmp/huker/agent01/myid case, we should write directly.
		if filepath.Base(fname) == fname {
			fname = path.Join(p.getJobRootDir(agentRootDir), CONF_DIR, fname)
		}
		if err := ioutil.WriteFile(fname, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// Render the agent root directory for config files and arguments.
func (p *Program) renderVars(agentRootDir string) {
	newConfigMap := make(map[string]string)
	for fname, content := range p.Configs {
		content = strings.Replace(content, "$AgentRootDir", agentRootDir, -1)
		content = strings.Replace(content, "$TaskId", strconv.Itoa(p.TaskId), -1)
		fname = strings.Replace(fname, "$AgentRootDir", agentRootDir, -1)
		fname = strings.Replace(fname, "$TaskId", strconv.Itoa(p.TaskId), -1)
		newConfigMap[fname] = content
	}
	p.Configs = newConfigMap

	for idx, arg := range p.Args {
		arg = strings.Replace(arg, "$AgentRootDir", agentRootDir, -1)
		arg = strings.Replace(arg, "$TaskId", strconv.Itoa(p.TaskId), -1)
		p.Args[idx] = arg
	}

	p.Bin = strings.Replace(p.Bin, "$AgentRootDir", agentRootDir, -1)
	p.Bin = strings.Replace(p.Bin, "$TaskId", strconv.Itoa(p.TaskId), -1)

	p.RootDir = p.getJobRootDir(agentRootDir)
}

// Install the packages and dump the configuration files for the process to start.
func (p *Program) Install(agentRootDir string) error {
	jobRootDir := p.getJobRootDir(agentRootDir)

	// step.0 Prev-check
	if relDir, err := filepath.Rel(agentRootDir, jobRootDir); err != nil {
		return err
	} else if strings.Contains(relDir, "..") || agentRootDir == jobRootDir {
		return fmt.Errorf("Permission denied, mkdir %s", jobRootDir)
	}
	if _, err := os.Stat(jobRootDir); err == nil {
		return fmt.Errorf("%s already exists, cleanup it first please.", jobRootDir)
	}

	// step.1 Create directories recursively
	for _, sub := range progDirs() {
		if err := os.MkdirAll(path.Join(jobRootDir, sub), 0755); err != nil {
			return err
		}
	}

	// step.2 Download package and link pkg to library.
	if err := p.updatePackage(agentRootDir); err != nil {
		return err
	}

	// step.3 Dump configuration files
	return p.dumpConfigFiles(agentRootDir)
}

// Start the process in daemon.
// TODO pipe stdout & stderr into pkg_root_dir/stdout directories.
func (p *Program) Start(s *Supervisor) error {
	if isProcessOK(p.PID) {
		return fmt.Errorf("Process %d is already running.", p.PID)
	}
	f, err := os.Create(path.Join(p.getJobRootDir(s.rootDir), STDOUT_DIR, "stdout"))
	if err != nil {
		return err
	}
	cmd := exec.Command(p.Bin, p.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		Pgid:   0,
	}
	cmd.Stdout, cmd.Stderr = f, f

	log.Debugf("Start to run command : [%s %s]", p.Bin, strings.Join(p.Args, " "))
	go func() {
		defer f.Close()
		if err := cmd.Run(); err != nil {
			log.Errorf("Run job failed. [cmd: %s %s], err: %v", p.Bin, strings.Join(p.Args, " "), err)
		}
	}()
	time.Sleep(time.Second * 1)

	if cmd.Process != nil && isProcessOK(cmd.Process.Pid) {
		log.Infof("Start process success. [%s %s]", p.Bin, strings.Join(p.Args, " "))
		p.Status = StatusRunning
		p.PID = cmd.Process.Pid
		return nil
	}
	return fmt.Errorf("Start job failed.")
}

// Stop the process.
func (p *Program) Stop(s *Supervisor) error {
	process, err := os.FindProcess(p.PID)
	if err != nil {
		return err
	}
	err = process.Kill()
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	// check the pid in the final
	if isProcessOK(p.PID) {
		return fmt.Errorf("Failed to stop the process %d, still running.", p.PID)
	}
	p.Status = StatusStopped
	return nil
}

// Restart the process
func (p *Program) Restart(s *Supervisor) error {
	p.Stop(s)
	if isProcessOK(p.PID) {
		return fmt.Errorf("Failed to stop the process %d, still running.", p.PID)
	}
	return p.Start(s)
}

func (p *Program) hookEnv() []string {
	var env []string
	env = append(env, "SUPERVISOR_ROOT_DIR="+path.Dir(path.Dir(p.RootDir)))
	env = append(env, "PROGRAM_BIN="+p.Bin)
	env = append(env, "PROGRAM_ARGS="+strings.Join(p.Args, " "))
	env = append(env, "PROGRAM_DIR="+p.RootDir)
	env = append(env, "PROGRAM_NAME="+p.Name)
	env = append(env, "PROGRAM_JOB_NAME="+p.Job)
	env = append(env, "PROGRAM_TASK_ID="+strconv.Itoa(p.TaskId))
	env = append(env, os.Environ()...)
	return env
}

// Execute hooks script in supervisor agent.
func (p *Program) ExecHooks(hook string) error {
	if _, ok := p.Hooks[hook]; !ok {
		return nil
	}
	log.Infof("Execute hook: %s", hook)
	// <agent-root-dir>/.hooks/<cluster>/<job>.<taskId>
	hooksDir := path.Join(path.Dir(path.Dir(p.RootDir)), HOOKS_DIR, p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId))
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	hookFile := path.Join(hooksDir, hook)
	if err := ioutil.WriteFile(hookFile, []byte(p.Hooks[hook]), 0755); err != nil {
		return err
	}
	// Execute the hooked bash script.
	return RunCommand(hookFile, p.hookEnv(), []string{}...)
}
