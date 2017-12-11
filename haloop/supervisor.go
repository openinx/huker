package haloop

import (
    "net/http"
    "github.com/gorilla/mux"
    "github.com/qiniu/log"
    "fmt"
    "encoding/json"
    "os"
    "path"
    "io"
    "os/exec"
    "bytes"
    "strings"
    "time"
    "io/ioutil"
    "html/template"
)

const (
    CODE_OK = 0
    CODE_FAIL = 1
    DATA_DIR = "data"
    LOG_DIR = "log"
    PKG_DIR = "pkg"
    CONF_DIR = "conf"
    STDOUT_DIR = "stdout"
)

func progDirs() []string {
    return []string{DATA_DIR, LOG_DIR, PKG_DIR, CONF_DIR, STDOUT_DIR}
}

type Supervisor struct {
    rootDir  string
    port     int
    dbFile   string
    programs []Program
}

type Program struct {
    Name       string `json:"name"`
    Job        string `json:"job"`
    Bin        string `json:"bin"`
    Args       []string `json:"args"`
    Configs    map[string]string `json:"configs"`
    PkgAddress string `json:"pkg_address"`
    PkgName    string `json:"pkg_name"`
    PkgMD5Sum  string `json:"pkg_md5sum"`
    PID        int `json:"pid"`
    Status     string `json:"status"`
}

func (p *Program) bootstrap(s *Supervisor) error {
    proRootDir := path.Join(s.rootDir, p.Name)

    // step.1 create directories recursively
    if err := os.MkdirAll(proRootDir, 0755); err != nil {
        return err
    }
    for _, sub := range progDirs() {
        if err := os.MkdirAll(path.Join(proRootDir, sub), 0755); err != nil {
            return err
        }
    }

    // step.2 download the package
    pkgFilePath := path.Join(proRootDir, PKG_DIR, p.PkgName)
    resp, err := http.Get(p.PkgAddress)
    if err != nil {
        log.Errorf("Downloading package failed. package : %s, err: %s", p.PkgAddress, err.Error())
        return err
    }
    defer resp.Body.Close()
    out, err := os.Create(pkgFilePath)
    if err != nil {
        log.Errorf("Create package file error: %v", err)
        return err
    }
    defer out.Close()
    io.Copy(out, resp.Body)

    // step.3 verify md5 checksum
    // TODO reuse those codes with pkg_manager.go
    md5sum := calcFileMD5Sum(pkgFilePath)
    if md5sum != p.PkgMD5Sum {
        return fmt.Errorf("md5sum mismatch, %s != %s, package: %s", md5sum, p.PkgMD5Sum, p.PkgName)
    }

    // step.4 extract package
    tarCmd := []string{"tar", "xzvf", pkgFilePath, "-C", path.Join(proRootDir, PKG_DIR)}
    cmd := exec.Command(tarCmd[0], tarCmd[1:]...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout, cmd.Stderr = &stdout, &stderr
    if err := cmd.Run(); err != nil {
        log.Errorf("exec cmd failed. [cmd: %s], [stdout: %s], [stderr: %s]",
            strings.Join(tarCmd, " "), stdout.String(), stderr.String())
        return err
    }

    // step.5 dump configuration files
    for fname, content := range p.Configs {
        fpath := path.Join(proRootDir, CONF_DIR, fname)
        out, err := os.Create(fpath)
        if err != nil {
            log.Errorf("save configuration file error: %v", err)
            return err
        }
        defer out.Close()
        io.Copy(out, bytes.NewBufferString(content))
    }

    // step.6 start the job
    cmd = exec.Command(p.Bin, p.Args...)
    stdout.Reset()
    stderr.Reset()
    cmd.Stdout, cmd.Stderr = &stdout, &stderr
    if err := cmd.Run(); err != nil {
        log.Errorf("exec cmd failed. [cmd: %s %s], [stdout: %s], [stderr: %s]",
            p.Bin, strings.Join(p.Args, " "), stdout.String(), stderr.String())
        return err
    }else {
        p.PID = cmd.Process.Pid
    }

    // step.7 update supervisor db file
    s.programs = append(s.programs, *p)
    if err := s.dumpSupervisorDB(); err != nil {
        return err
    }
    return nil
}

func (p *Program) start(s *Supervisor) error {
    var stdout, stderr bytes.Buffer
    cmd := exec.Command(p.Bin, p.Args...)
    stdout.Reset()
    stderr.Reset()
    cmd.Stdout, cmd.Stderr = &stdout, &stderr
    if err := cmd.Run(); err != nil {
        log.Errorf("exec cmd failed. [cmd: %s %s], [stdout: %s], [stderr: %s]",
            p.Bin, strings.Join(p.Args, " "), stdout.String(), stderr.String())
        return err
    }else {
        p.PID = cmd.Process.Pid
    }
    return nil
}

func (p *Program) stop(s *Supervisor) error {
    return nil
}

func (p *Program) cleanup(s *Supervisor) error {
    return nil
}

func (p *Program) restart(s *Supervisor) error {
    return nil
}

func (p *Program) rollingUpdate(s *Supervisor) error {
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
			<td>Running</td>
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
    message := "success"
    if err != nil {
        code = CODE_FAIL
        message = fmt.Sprintf("error: %v", err)
    }
    data, _ := json.Marshal(map[string]interface{}{
        "status": code,
        "message": message,
    })
    return data
}

func (s *Supervisor) hBootstrapProgram(w http.ResponseWriter, r *http.Request) {
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        w.Write(renderResp(err))
    }
    prog := &Program{}
    err = json.Unmarshal(body, prog)

    log.Infof(prog.Name)

    if err != nil {
        w.Write(renderResp(err))
    }
    w.Write(renderResp(prog.bootstrap(s)))
}

func (s *Supervisor) getProgram(name, job string) *Program {
    for _, prog := range s.programs {
        if prog.Name == name && prog.Job == job {
            return &prog
        }
    }
    return nil
}

func (s *Supervisor) hShowProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        data, err := json.Marshal(prog)
        if err != nil {
            w.Write(renderResp(err))
        }else {
            io.Copy(w, bytes.NewBuffer(data))
        }
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
}

func (s *Supervisor) hStartProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        w.Write(renderResp(prog.start(s)))
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
}

func (s *Supervisor) hCleanupProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        w.Write(renderResp(prog.cleanup(s)))
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
}

func (s *Supervisor) hUpdateProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        w.Write(renderResp(prog.rollingUpdate(s)))
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
}

func (s *Supervisor) hRestartProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        w.Write(renderResp(prog.restart(s)))
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
}

func (s *Supervisor) hStopProgram(w http.ResponseWriter, r *http.Request) {
    // TODO Abstract show/start/cleanup/rolling_update/restart/stop logic
    name := mux.Vars(r)["name"]
    job := mux.Vars(r)["job"]
    if prog := s.getProgram(name, job); prog != nil {
        w.Write(renderResp(prog.stop(s)))
    }else {
        w.Write(renderResp(fmt.Errorf("name: %s, job: %s not found.", name, job)))
    }
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

    // check the status of program
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

func NewSupervisor() *Supervisor {
    s := &Supervisor{
        rootDir: fmt.Sprintf("/Users/openinx/test/%d", int32(time.Now().Unix())),
        port:9001,
        dbFile:"./supervisor.db",
        programs: []Program{},
    }
    // TODO return to up layer.
    if err := s.loadSupervisorDB(); err != nil {
        log.Errorf("%v", err)
    }
    return s
}

func (s *Supervisor) Start() {
    r := mux.NewRouter()
    r.HandleFunc("/", s.hIndex)
    r.HandleFunc("/api/programs", s.hGetProgramList).Methods("GET")
    r.HandleFunc("/api/programs", s.hBootstrapProgram).Methods("POST")
    r.HandleFunc("/api/programs/{name}/{job}", s.hShowProgram).Methods("GET")
    r.HandleFunc("/api/programs/{name}/{job}/start", s.hStartProgram).Methods("POST")
    r.HandleFunc("/api/programs/{name}/{job}", s.hUpdateProgram).Methods("PUT")
    r.HandleFunc("/api/programs/{name}/{job}", s.hRestartProgram).Methods("PUT")
    r.HandleFunc("/api/programs/{name}/{job}", s.hCleanupProgram).Methods("DELETE")
    r.HandleFunc("/api/programs/{name}/{job}/stop", s.hStopProgram).Methods("POST")
    http.Handle("/", r)
    if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), nil); err != nil {
        log.Errorf("%v", err)
    }
}
