package huker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

type ProgramMap struct {
	mux      sync.Mutex
	programs map[string]Program // Map hash of (cluster, job, taskId) to program instance.
}

func programHash(cluster, job string, taskId int) string {
	return fmt.Sprintf("cluster=%s/job=%s/task_id=%d", cluster, job, taskId)
}

func NewProgramMap() *ProgramMap {
	return &ProgramMap{
		programs: make(map[string]Program),
	}
}

func (p *ProgramMap) Get(cluster, job string, taskId int) (Program, bool) {
	key := programHash(cluster, job, taskId)
	prog, ok := p.programs[key]
	return prog, ok
}

func (p *ProgramMap) PutAndDump(prog *Program, fileName string) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	// Put it into map
	key := programHash(prog.Name, prog.Job, prog.TaskId)
	p.programs[key] = *prog

	// Dump it to file
	return p.DumpToFile(fileName)
}

func (p *ProgramMap) RefreshAndDump(fileName string) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	for key, prog := range p.programs {
		if isProcessOK(prog.PID) {
			prog.Status = StatusRunning
		} else {
			prog.Status = StatusStopped
		}
		p.programs[key] = prog
	}

	return p.DumpToFile(fileName)
}

func (p *ProgramMap) DumpToFile(fileName string) error {
	// Open file, create it if not exist.
	f, createErr := os.Create(fileName)
	if createErr != nil {
		return createErr
	}
	defer f.Close()

	// Marshal the map and dump to the file
	data, err := p.Marshal()
	if err != nil {
		return err
	}
	_, err = io.Copy(f, bytes.NewBuffer(data))
	return err
}

func (p *ProgramMap) Remove(prog *Program) {
	p.mux.Lock()
	p.mux.Unlock()
	delete(p.programs, programHash(prog.Name, prog.Job, prog.TaskId))
}

func (p *ProgramMap) Marshal() ([]byte, error) {
	return json.Marshal(p.programs)
}

func (p *ProgramMap) toArray() []Program {
	p.mux.Lock()
	defer p.mux.Unlock()
	var progArray []Program
	for _, prog := range p.programs {
		progArray = append(progArray, prog)
	}
	sort.Slice(progArray, func(i, j int) bool {
		c0 := strings.Compare(progArray[i].Name, progArray[j].Name)
		if c0 != 0 {
			return c0 < 0
		}
		c0 = strings.Compare(progArray[i].Job, progArray[j].Job)
		if c0 != 0 {
			return c0 < 0
		}
		return progArray[i].PID < progArray[j].PID

	})
	return progArray
}
