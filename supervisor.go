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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	CODE_OK         = 0
	CODE_FAIL       = 1
	MESSAGE_SUCCESS = "success"
)

type Supervisor struct {
	rootDir       string
	port          int
	dbFile        string
	programs      *ProgramMap
	quit          chan int
	refreshTicker *time.Ticker
	srv           *http.Server
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
	s.updateProgram(w, r, func(p *Program) error {
		// Step.0 check the existence of program.
		if _, ok := s.programs.Get(p.Name, p.Job, p.TaskId); ok {
			return fmt.Errorf("Job %s.%s.%d already exists.", p.Name, p.Job, p.TaskId)
		}
		// Step.1 Execute prev bootstrap hook
		if err := p.ExecHooks("pre_bootstrap"); err != nil {
			return err
		}
		// Step.2 Install package under root directory of agent.
		if err := p.Install(s.rootDir); err != nil {
			return err
		}
		// Step.3 Execute post bootstrap hook
		return p.ExecHooks("post_bootstrap")
	})
}

// Abstract method for bootstrap/rolling_update.
func (s *Supervisor) updateProgram(w http.ResponseWriter, r *http.Request, handleFunc func(*Program) error) {
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

	prog.renderVars(s.rootDir)
	if err := handleFunc(prog); err != nil {
		w.Write(renderResp(err))
		return
	}

	// Defer to update supervisor db file, whether start job success or not.
	defer func() {
		if err := s.programs.PutAndDump(prog, s.dbFile); err != nil {
			log.Errorf("Failed to dump supervisor db files: %v", err)
		}
	}()

	// Start the job in the final.
	prog.Status = StatusStopped
	w.Write(renderResp(prog.Start(s)))
}

// Abstract method for start/cleanup/restart/stop.
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
		w.Write(renderResp(fmt.Errorf("name: %s, job: %s, taskId: %d not found.", name, job, taskId)))
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
		w.Write(renderResp(fmt.Errorf("name: %s, job: %s, taskId: %d not found.", name, job, taskId)))
	}
}

func (s *Supervisor) hStartProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		if err := p.ExecHooks("pre_start"); err != nil {
			return err
		}
		if err := p.Start(s); err != nil {
			return err
		}
		return p.ExecHooks("post_start")
	})
}

// Be Careful: Forbidden to let user delete the root.
func (s *Supervisor) hCleanupProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	job := mux.Vars(r)["job"]
	taskId, _ := strconv.Atoi(mux.Vars(r)["taskId"])

	// step.0 check and clear cache.
	prog, progFound := s.programs.Get(name, job, taskId)
	if progFound {
		if prog.Status == StatusRunning {
			w.Write(renderResp(fmt.Errorf("Job %s.%s.%d is still running, stop it first please.",
				prog.Name, prog.Job, prog.TaskId)))
			return
		}
		if err := prog.ExecHooks("pre_cleanup"); err != nil {
			w.Write(renderResp(err))
			return
		}
		s.programs.Remove(&prog)
	}

	// issue#3: when the job does not exist in supervisor cache, still need to cleanup the data. because
	// the supervisor may failed to start process when bootstrap and left the directory dir(pkg/data/conf..etc)

	// TODO abstract a common method to generate jobRootDir.
	jobRootDir := path.Join(s.rootDir, name, fmt.Sprintf("%s.%d", job, taskId))

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
	targetPath := path.Join(s.rootDir, name, fmt.Sprintf(".trash.%s.%d.%d", job, taskId, int32(time.Now().Unix())))
	if err := os.Rename(jobRootDir, targetPath); err != nil {
		w.Write(renderResp(err))
		return
	}

	// step.3 Execute post hook
	if progFound {
		if err := prog.ExecHooks("post_cleanup"); err != nil {
			w.Write(renderResp(err))
			return
		}
	}

	w.Write(renderResp(nil))
}

func (s *Supervisor) hRollingUpdateProgram(w http.ResponseWriter, r *http.Request) {
	s.updateProgram(w, r, func(p *Program) error {
		// Step.0 check the existence of program.
		if curProg, ok := s.programs.Get(p.Name, p.Job, p.TaskId); !ok {
			return fmt.Errorf("Bootstrap %s.%s.%d first please.", p.Name, p.Job, p.TaskId)
		} else {
			curProg.Stop(s)
		}
		// Step.1 Execute prev hook
		if err := p.ExecHooks("pre_rolling_update"); err != nil {
			return err
		}
		// Step.2 Update packages.
		if err := p.updatePackage(s.rootDir); err != nil {
			return err
		}
		// Step.3 Update config files.
		if err := p.updateConfigFiles(s.rootDir); err != nil {
			return err
		}
		// Step.4 Execute post hook
		return p.ExecHooks("post_rolling_update")
	})
}

func (s *Supervisor) hRestartProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		if err := p.ExecHooks("pre_restart"); err != nil {
			return err
		}
		if err := p.Restart(s); err != nil {
			return err
		}
		return p.ExecHooks("post_restart")
	})
}

func (s *Supervisor) hStopProgram(w http.ResponseWriter, r *http.Request) {
	s.handleProgram(w, r, func(p *Program) error {
		if err := p.ExecHooks("pre_stop"); err != nil {
			return err
		}
		if err := p.Stop(s); err != nil {
			return err
		}
		return p.ExecHooks("post_stop")
	})
}

func (s *Supervisor) loadSupervisorDB() error {
	// step.0 Create if not exist
	if _, err := os.Stat(s.dbFile); os.IsNotExist(err) {
		log.Infof("%s does not exist, initialize to be empty program list.", s.dbFile)
		return s.programs.DumpToFile(s.dbFile)
	}

	// step.1 Load programs from file and unmarshal it.
	bytes, err := ioutil.ReadFile(s.dbFile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bytes, &(s.programs.programs)); err != nil {
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
		quit:          make(chan int),
		refreshTicker: time.NewTicker(1 * time.Second),
		srv: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
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
	s.srv.Handler = r
	return s.srv.ListenAndServe()
}

func (s *Supervisor) Stop() error {
	s.quit <- 1
	return s.srv.Close()
}
