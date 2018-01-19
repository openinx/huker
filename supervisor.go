package huker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/qiniu/log"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	CODE_OK         = 0
	CODE_FAIL       = 1
	MESSAGE_SUCCESS = "success"
	DATA_DIR        = "data"
	LOG_DIR         = "log"
	PKG_DIR         = "pkg"
	CONF_DIR        = "conf"
	STDOUT_DIR      = "stdout"
	StatusRunning   = "Running"
	StatusStopped   = "Stopped"
)

func progDirs() []string {
	return []string{DATA_DIR, LOG_DIR, CONF_DIR, STDOUT_DIR}
}

type Supervisor struct {
	rootDir       string
	port          int
	dbFile        string
	programs      *ProgramMap
	quit          chan struct{}
	refreshTicker *time.Ticker
}

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
}

func (p *Program) install(agentRootDir string) error {
	jobRootDir := path.Join(agentRootDir, p.Name, fmt.Sprintf("%s.%d", p.Job, p.TaskId))

	// step.0 render the agent root directory for config files and arguments.
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
	p.RootDir = jobRootDir

	// step.1 prev-check
	if relDir, err := filepath.Rel(agentRootDir, jobRootDir); err != nil {
		return err
	} else if strings.Contains(relDir, "..") || agentRootDir == jobRootDir {
		return fmt.Errorf("Permission denied, mkdir %s", jobRootDir)
	}
	if _, err := os.Stat(jobRootDir); err == nil {
		return fmt.Errorf("%s is already exists, cleanup it first please.", jobRootDir)
	}

	// step.2 create directories recursively
	if err := os.MkdirAll(jobRootDir, 0755); err != nil {
		return err
	}
	for _, sub := range progDirs() {
		if err := os.MkdirAll(path.Join(jobRootDir, sub), 0755); err != nil {
			return err
		}
	}

	// step.3 download the package
	pkgFilePath := path.Join(jobRootDir, STDOUT_DIR, p.PkgName)
	resp, err := http.Get(p.PkgAddress)
	if err != nil {
		log.Errorf("Downloading package failed. package : %s, err: %s", p.PkgAddress, err.Error())
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Errorf("Downloading package failed. package : %s, err: %s", p.PkgAddress, resp.Status)
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(data))
	}
	out, err := os.Create(pkgFilePath)
	if err != nil {
		log.Errorf("Create package file error: %v", err)
		return err
	}
	defer out.Close()
	io.Copy(out, resp.Body)

	// step.4 verify md5 checksum
	// TODO reuse those codes with pkgsrv.go
	md5sum, md5Err := calcFileMD5Sum(pkgFilePath)
	if md5Err != nil {
		log.Errorf("Calculate the md5 checksum of file %s failed, cause: %v", pkgFilePath, md5Err)
		return md5Err
	}
	if md5sum != p.PkgMD5Sum {
		return fmt.Errorf("md5sum mismatch, %s != %s, package: %s", md5sum, p.PkgMD5Sum, p.PkgName)
	}

	// step.5 extract package
	tarCmd := []string{"tar", "xzvf", pkgFilePath, "-C", path.Join(jobRootDir, STDOUT_DIR)}
	cmd := exec.Command(tarCmd[0], tarCmd[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("exec cmd failed. [cmd: %s], [stdout: %s], [stderr: %s]",
			strings.Join(tarCmd, " "), stdout.String(), stderr.String())
		return err
	}
	// step.6 Move all files under <pkgRootDir>/<pkg-prefix-dir> to <pkgRootDir>
	if idx := strings.LastIndex(p.PkgName, ".tar.gz"); idx >= 0 {
		pkgRootDir := path.Join(jobRootDir, STDOUT_DIR)
		pkgPrefixDir := path.Join(pkgRootDir, p.PkgName[0:idx])
		if _, err := os.Stat(pkgPrefixDir); os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist, skip to move all files under it to %s", pkgPrefixDir, pkgRootDir)
		} else {
			realPkgRootDir := path.Join(jobRootDir, PKG_DIR)
			if _, err := os.Stat(realPkgRootDir); os.IsExist(err) {
				os.RemoveAll(realPkgRootDir)
			}
			if err := os.Symlink(pkgPrefixDir, realPkgRootDir); err != nil {
				return err
			}
		}
	}

	// step.7 dump configuration files
	for fname, content := range p.Configs {
		// When fname is /tmp/huker/agent01/myid case, we should write directly.
		cfgPath := fname
		if !strings.Contains(fname, "/") {
			cfgPath = path.Join(jobRootDir, CONF_DIR, fname)
		}
		out, err := os.Create(cfgPath)
		if err != nil {
			log.Errorf("save configuration file error: %v", err)
			return err
		}
		defer out.Close()
		io.Copy(out, bytes.NewBufferString(content))
	}

	return nil
}

// TODO pipe stdout & stderr into pkg_root_dir/stdout directories.
func (p *Program) start(s *Supervisor) error {
	if isProcessOK(p.PID) {
		return fmt.Errorf("Process %d is already running.", p.PID)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(p.Bin, p.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		Pgid:   0,
	}
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	// TODO handle the ERROR, otherwise cmd.Process will panic because of NULL pointer.
	go func() {
		if err := cmd.Start(); err != nil {
			log.Errorf("Start job failed. [cmd: %s %s], [stdout: %s], [stderr: %s], err: %v",
				p.Bin, strings.Join(p.Args, " "), stdout.String(), stderr.String(), err)
		}
		if err := cmd.Wait(); err != nil {
			log.Errorf("Wait job failed. [cmd: %s %s], [stdout: %s], [stderr: %s], err: %v",
				p.Bin, strings.Join(p.Args, " "), stdout.String(), stderr.String(), err)
		}
	}()
	time.Sleep(time.Second * 1)

	if cmd.Process != nil && isProcessOK(cmd.Process.Pid) {
		p.Status = StatusRunning
		p.PID = cmd.Process.Pid
		return nil
	} else {
		return fmt.Errorf("Start job failed.")
	}
}

func (p *Program) stop(s *Supervisor) error {
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

func (p *Program) restart(s *Supervisor) error {
	p.stop(s)
	if isProcessOK(p.PID) {
		// TODO check process status
		return fmt.Errorf("Failed to stop the process %d, still running.", p.PID)
	}
	err := p.start(s)
	return err
}

func (p *Program) rollingUpdate(s *Supervisor) error {
	// TODO
	return nil
}

func (s *Supervisor) hIndex(w http.ResponseWriter, r *http.Request) {
	const INDEX_TMPL = `
	<table border="1" bordercolor="#a0c6e5" style="border-collapse:collapse;" align="left">
		<tr>
			<td>Name</td>
			<td>Job</td>
			<td>PID</td>
			<td>Status</td>
		</tr>

		{{ range .}}
		<tr>
			<td>{{ .Name }}</td>
			<td>{{ .Job }}</td>
			<td>{{ .PID }}</td>
			<td>{{ .Status }}</td>
		</tr>
		{{ end }}
	</table>
	<div style="clear:both">
	{{ len . }} programs in total.
	`

	t, err := template.New("Get Program List").Parse(INDEX_TMPL)
	if err != nil {
		log.Error("Parse template failed: %v", err)
		return
	}
	if err := t.Execute(w, s.programs.toArray()); err != nil {
		log.Errorf("Render template error: %v", err)
	}
}

func (s *Supervisor) hGetProgramList(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(s.programs.toArray())
	if err != nil {
		w.Write(renderResp(err))
	}
	io.Copy(w, bytes.NewBuffer(data))
}

func renderResp(err error) []byte {
	code := CODE_OK
	message := MESSAGE_SUCCESS
	if err != nil {
		code = CODE_FAIL
		message = fmt.Sprintf("error: %v", err)
	}
	data, _ := json.Marshal(map[string]interface{}{
		"status":  code,
		"message": message,
	})
	return data
}

func (s *Supervisor) hBootstrapProgram(w http.ResponseWriter, r *http.Request) {
	// Step.0 Read and unmarshal the program.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.Write(renderResp(err))
		return
	}
	prog := &Program{}
	err = json.Unmarshal(body, prog)
	if err != nil {
		w.Write(renderResp(err))
		return
	}

	// Step.1 check the existence of program.
	if p, ok := s.programs.Get(prog.Name, prog.Job, prog.TaskId); ok {
		w.Write(renderResp(fmt.Errorf("Job %s.%s.%s already exists.", p.Name, p.Job, p.TaskId)))
		return
	}

	// Step.2 Install package under root directory of agent.
	err = prog.install(s.rootDir)
	if err != nil {
		w.Write(renderResp(err))
		return
	}

	// Defer to update supervisor db file, whether start job success or not.
	defer func() {
		if err := s.programs.PutAndDump(prog, s.dbFile); err != nil {
			log.Errorf("Failed to dump supervisor db files: %v", err)
		}
	}()

	// Step.4 Start the job
	prog.Status = StatusStopped
	w.Write(renderResp(prog.start(s)))
}

// Abstract method for start/cleanup/rolling_update/restart/stop logic
func (s *Supervisor) handleProgram(w http.ResponseWriter, r *http.Request, handleFunc func(*Program) error) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	taskId, _ := strconv.Atoi(mux.Vars(r)["taskId"])

	if prog, ok := s.programs.Get(name, job, taskId); ok {
		if err := handleFunc(&prog); err != nil {
			w.Write(renderResp(err))
			return
		}

		// Keep the latest version of program saved in supervisor.
		w.Write(renderResp(s.programs.PutAndDump(&prog, s.dbFile)))
	} else {
		w.Write(renderResp(fmt.Errorf("name: %s, job: %s, taskId: %s not found.", name, job, taskId)))
	}
}

func (s *Supervisor) hShowProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	taskId, _ := strconv.Atoi(mux.Vars(r)["taskId"])
	if prog, ok := s.programs.Get(name, job, taskId); ok {
		data, err := json.Marshal(prog)
		if err != nil {
			w.Write(renderResp(err))
		} else {
			io.Copy(w, bytes.NewBuffer(data))
		}
	} else {
		w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
	}
}

func (s *Supervisor) hStartProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		return p.start(s)
	})
}

// Be Careful: Forbidden to let user delete the root.
func (s *Supervisor) hCleanupProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	taskId, _ := strconv.Atoi(mux.Vars(r)["taskId"])

	// step.0 check and clear cache.
	if prog, ok := s.programs.Get(name, job, taskId); ok {
		if prog.Status == StatusRunning {
			w.Write(renderResp(fmt.Errorf("Job %s.%s.%s is still running, stop it first please.",
				prog.Name, prog.Job, prog.TaskId)))
			return
		}
		s.programs.Remove(&prog)
	}

	// issue#3: when the job does not exist in supervisor cache, still need to cleanup the data. because
	// the supervisor may failed to start process when bootstrap and left the directory dir(pkg/data/conf..etc)

	// TODO abstract a common method to generate jobRootDir.
	jobRootDir := path.Join(s.rootDir, name, fmt.Sprintf("%s.%s", job, taskId))

	// step.1 check the job root dir
	if _, err := os.Stat(jobRootDir); os.IsNotExist(err) {
		w.Write(renderResp(fmt.Errorf("Root dir of job %s does not exist. no need to cleanup", jobRootDir)))
		return
	}
	relDir, err := filepath.Rel(s.rootDir, jobRootDir)
	if err != nil {
		w.Write(renderResp(err))
		return
	}
	if strings.Contains(relDir, "..") || s.rootDir == jobRootDir {
		w.Write(renderResp(fmt.Errorf("Cann't cleanup the directory %s, Permission Denied", jobRootDir)))
		return
	}

	// step.2 rename the job root dir into .trash
	targetPath := path.Join(s.rootDir, name, fmt.Sprintf(".trash.%s.%s.%d", job, taskId, int32(time.Now().Unix())))
	if err := os.Rename(jobRootDir, targetPath); err != nil {
		w.Write(renderResp(err))
		return
	}

	// step.3 return success.
	w.Write(renderResp(nil))
}

func (s *Supervisor) hRollingUpdateProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		return p.rollingUpdate(s)
	})
}

func (s *Supervisor) hRestartProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		return p.restart(s)
	})
}

func (s *Supervisor) hStopProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		return p.stop(s)
	})
}

func (s *Supervisor) loadSupervisorDB() error {
	// step.0 Create if not exist
	if _, err := os.Stat(s.dbFile); os.IsNotExist(err) {
		log.Infof("%s does not exist, initialize to be empty program list.", s.dbFile)
		return s.programs.DumpToFile(s.dbFile)
	}

	// step.1 Load programs from file
	f, err := os.Open(s.dbFile)
	if err != nil {
		return fmt.Errorf("Open %s error: %v", s.dbFile, err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return fmt.Errorf("Read %s error: %v", s.dbFile, err)
	}
	if err := json.Unmarshal(buf.Bytes(), &(s.programs.programs)); err != nil {
		return fmt.Errorf("Unmarshal %s error: %v", s.dbFile, err)
	}

	return nil
}

func NewSupervisor(rootDir string, port int, supervisorDB string) (*Supervisor, error) {
	s := &Supervisor{
		rootDir:       rootDir,
		port:          port,
		dbFile:        supervisorDB,
		programs:      NewProgramMap(),
		quit:          make(chan struct{}),
		refreshTicker: time.NewTicker(1 * time.Second),
	}

	// Load supervisor db
	if err := s.loadSupervisorDB(); err != nil {
		return nil, err
	}

	// Start the period refresh task
	go func() {
		for {
			select {
			case <-s.refreshTicker.C:
				if err := s.programs.RefreshAndDump(s.dbFile); err != nil {
					log.Errorf("Failed to refresh and dump %s, %s", s.dbFile, err)
				}
			case <-s.quit:
				s.refreshTicker.Stop()
				return
			}
		}
	}()
	return s, nil
}

func (s *Supervisor) Start() error {
	r := mux.NewRouter()
	r.HandleFunc("/", s.hIndex)
	r.HandleFunc("/api/programs", s.hGetProgramList).Methods("GET")
	r.HandleFunc("/api/programs", s.hBootstrapProgram).Methods("POST")
	r.HandleFunc("/api/programs/{name}/{job}/{taskId}", s.hShowProgram).Methods("GET")
	r.HandleFunc("/api/programs/{name}/{job}/{taskId}/start", s.hStartProgram).Methods("PUT")
	r.HandleFunc("/api/programs/rolling_update", s.hRollingUpdateProgram).Methods("POST")
	r.HandleFunc("/api/programs/{name}/{job}/{taskId}/restart", s.hRestartProgram).Methods("PUT")
	r.HandleFunc("/api/programs/{name}/{job}/{taskId}", s.hCleanupProgram).Methods("DELETE")
	r.HandleFunc("/api/programs/{name}/{job}/{taskId}/stop", s.hStopProgram).Methods("PUT")
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: r,
	}
	return srv.ListenAndServe()
}
