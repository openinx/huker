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
	programs      []Program
	quit          chan struct{}
	refreshTicker *time.Ticker
}

// TODO lock for modification of status
type Program struct {
	Name       string            `json:"name"`
	Job        string            `json:"job"`
	Bin        string            `json:"bin"`
	Args       []string          `json:"args"`
	Configs    map[string]string `json:"configs"`
	PkgAddress string            `json:"pkg_address"`
	PkgName    string            `json:"pkg_name"`
	PkgMD5Sum  string            `json:"pkg_md5sum"`
	PID        int               `json:"pid"`
	Status     string            `json:"status"`
}

func (p *Program) bootstrap(s *Supervisor) error {
	jobRootDir := path.Join(s.rootDir, p.Name, p.Job)

	// step.0 prev-check
	if relDir, err := filepath.Rel(s.rootDir, jobRootDir); err != nil {
		return err
	} else if strings.Contains(relDir, "..") || s.rootDir == jobRootDir {
		return fmt.Errorf("Permission denied, mkdir %s", jobRootDir)
	}
	if _, err := os.Stat(jobRootDir); os.IsExist(err) {
		return fmt.Errorf("%s is already exists, cleanup it first please.", jobRootDir)
	}

	// step.1 create directories recursively
	if err := os.MkdirAll(jobRootDir, 0755); err != nil {
		return err
	}
	for _, sub := range progDirs() {
		if err := os.MkdirAll(path.Join(jobRootDir, sub), 0755); err != nil {
			return err
		}
	}

	// step.2 download the package
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

	// step.3 verify md5 checksum
	// TODO reuse those codes with pkgsrv.go
	md5sum, md5Err := calcFileMD5Sum(pkgFilePath)
	if md5Err != nil {
		log.Errorf("Calculate the md5 checksum of file %s failed, cause: %v", pkgFilePath, md5Err)
		return md5Err
	}
	if md5sum != p.PkgMD5Sum {
		return fmt.Errorf("md5sum mismatch, %s != %s, package: %s", md5sum, p.PkgMD5Sum, p.PkgName)
	}

	// step.4 extract package
	tarCmd := []string{"tar", "xzvf", pkgFilePath, "-C", path.Join(jobRootDir, STDOUT_DIR)}
	cmd := exec.Command(tarCmd[0], tarCmd[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("exec cmd failed. [cmd: %s], [stdout: %s], [stderr: %s]",
			strings.Join(tarCmd, " "), stdout.String(), stderr.String())
		return err
	}
	// step.5 Move all files under <pkgRootDir>/<pkg-prefix-dir> to <pkgRootDir>
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

	// step.6 dump configuration files
	for fname, content := range p.Configs {
		cfgPath := path.Join(jobRootDir, CONF_DIR, fname)
		out, err := os.Create(cfgPath)
		if err != nil {
			log.Errorf("save configuration file error: %v", err)
			return err
		}
		defer out.Close()
		io.Copy(out, bytes.NewBufferString(content))
	}

	// step.7 start the job
	if err := p.start(s); err != nil {
		return err
	}

	// step.8 update supervisor db file
	s.programs = append(s.programs, *p)
	if err := s.dumpSupervisorDB(); err != nil {
		return err
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

	p.PID = cmd.Process.Pid
	if isProcessOK(cmd.Process.Pid) {
		p.Status = StatusRunning
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
	if err := t.Execute(w, s.programs); err != nil {
		log.Errorf("Render template error: %v", err)
	}
}

func (s *Supervisor) hGetProgramList(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(s.programs)
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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.Write(renderResp(err))
		return
	}
	prog := &Program{}
	err = json.Unmarshal(body, prog)
	prog.Status = StatusStopped

	log.Infof(prog.Name)

	if err != nil {
		w.Write(renderResp(err))
		return
	}
	for _, p := range s.programs {
		if prog.Name == p.Name && prog.Job == p.Job {
			w.Write(renderResp(fmt.Errorf("Job %s.%s already exists.", prog.Name, prog.Job)))
			return
		}
	}

	if err = prog.bootstrap(s); err != nil {
		w.Write(renderResp(err))
		return
	}
	w.Write(renderResp(nil))
}

func (s *Supervisor) getProgram(name, job string) *Program {
	for _, prog := range s.programs {
		if prog.Name == name && prog.Job == job {
			return &prog
		}
	}
	return nil
}

// Abstract method for start/cleanup/rolling_update/restart/stop logic
func (s *Supervisor) handleProgram(w http.ResponseWriter, r *http.Request, handleFunc func(*Program) error) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	if prog := s.getProgram(name, job); prog != nil {
		if err := handleFunc(prog); err != nil {
			w.Write(renderResp(err))
			return
		}
		// Keep the latest version of program save in supervisor.
		// TODO check the similar case.
		for i := range s.programs {
			if s.programs[i].Name == prog.Name && s.programs[i].Job == prog.Job {
				s.programs[i] = *prog
			}
		}
		if err := s.dumpSupervisorDB(); err != nil {
			w.Write(renderResp(err))
			return
		}
		w.Write(renderResp(nil))
	} else {
		w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
	}
}

func (s *Supervisor) hShowProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	if prog := s.getProgram(name, job); prog != nil {
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
	s.handleProgram(w, r, func(p *Program) error {
		jobRootDir := path.Join(s.rootDir, p.Name, p.Job)
		// step.1 check the job root dir
		if _, err := os.Stat(jobRootDir); os.IsNotExist(err) {
			return fmt.Errorf("Root dir of job %s does not exist.", jobRootDir)
		}
		relDir, err := filepath.Rel(s.rootDir, jobRootDir)
		if err != nil {
			return err
		}
		if strings.Contains(relDir, "..") || s.rootDir == jobRootDir {
			return fmt.Errorf("Cann't cleanup the directory %s, Permission Denied", jobRootDir)
		}

		// step.2 rename the job root dir into .trash
		targetPath := path.Join(s.rootDir, p.Name, fmt.Sprintf(".trash.%s.%d", p.Job, int32(time.Now().Unix())))
		if err := os.Rename(jobRootDir, targetPath); err != nil {
			return err
		}

		// step.3 remove the program from cache.
		var programs []Program
		for _, prog := range s.programs {
			if prog.Name != p.Name && prog.Job != p.Job {
				programs = append(programs, prog)
			}
		}
		return nil
	})
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
	if _, err := os.Stat(s.dbFile); os.IsNotExist(err) {
		// create if not exist
		log.Infof("%s does not exist, initialize to be empty program list.", s.dbFile)
		out, err := os.Create(s.dbFile)
		if err != nil {
			return err
		}
		defer out.Close()
		data, _ := json.Marshal(s.programs)
		if _, err := io.Copy(out, bytes.NewBuffer(data)); err != nil {
			return err
		}
		return nil
	}
	f, err := os.Open(s.dbFile)
	if err != nil {
		return fmt.Errorf("Open %s error: %v", s.dbFile, err)
	}
	defer f.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return fmt.Errorf("Read %s error: %v", s.dbFile, err)
	}
	if err := json.Unmarshal(buf.Bytes(), &s.programs); err != nil {
		return fmt.Errorf("Deserilize %s error: %v", s.dbFile, err)
	}

	// TODO check current status of programs
	return nil
}

func (s *Supervisor) dumpSupervisorDB() error {
	// TODO write to a .tmp file first, then db file.
	f, createErr := os.Create(s.dbFile)
	if createErr != nil {
		return createErr
	}
	defer f.Close()
	data, err := json.Marshal(s.programs)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, bytes.NewBuffer(data)); err != nil {
		return err
	}
	return nil
}

func NewSupervisor(rootDir string, port int, supervisorDB string) (*Supervisor, error) {
	s := &Supervisor{
		rootDir:       rootDir,
		port:          port,
		dbFile:        supervisorDB,
		programs:      []Program{},
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
				for i := range s.programs {
					if isProcessOK(s.programs[i].PID) {
						s.programs[i].Status = StatusRunning
					} else {
						s.programs[i].Status = StatusStopped
					}
				}
				if err := s.dumpSupervisorDB(); err != nil {
					log.Warnf("Failed to refresh program's status, dump supervisor db error: %v", err)
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
	r.HandleFunc("/api/programs/{name}/{job}", s.hShowProgram).Methods("GET")
	r.HandleFunc("/api/programs/{name}/{job}/start", s.hStartProgram).Methods("PUT")
	r.HandleFunc("/api/programs/rolling_update", s.hRollingUpdateProgram).Methods("POST")
	r.HandleFunc("/api/programs/{name}/{job}/restart", s.hRestartProgram).Methods("PUT")
	r.HandleFunc("/api/programs/{name}/{job}", s.hCleanupProgram).Methods("DELETE")
	r.HandleFunc("/api/programs/{name}/{job}/stop", s.hStopProgram).Methods("PUT")
	srv := http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: r,
	}
	return srv.ListenAndServe()
}
