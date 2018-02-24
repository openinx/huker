package huker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
)

type programMap struct {
	mux      sync.Mutex
	programs map[string]Program // Map hash of (cluster, job, taskId) to program instance.
}

func programHash(cluster, job string, taskId int) string {
	return fmt.Sprintf("cluster=%s/job=%s/task_id=%d", cluster, job, taskId)
}

func newProgramMap() *programMap {
	return &programMap{
		programs: make(map[string]Program),
	}
}

func (p *programMap) get(cluster, job string, taskId int) (Program, bool) {
	key := programHash(cluster, job, taskId)
	prog, ok := p.programs[key]
	return prog, ok
}

func (p *programMap) putAndDump(prog *Program, fileName string) error {
	p.mux.Lock()
	defer p.mux.Unlock()

	// Put it into map
	key := programHash(prog.Name, prog.Job, prog.TaskId)
	p.programs[key] = *prog

	// Dump it to file
	return p.dumpToFile(fileName)
}

func (p *programMap) refreshAndDump(fileName string) error {
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

	return p.dumpToFile(fileName)
}

func (p *programMap) dumpToFile(fileName string) error {
	// Marshal the map and dump to the file
	data, err := json.Marshal(p.programs)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, 0644)
}

func (p *programMap) remove(prog *Program) {
	p.mux.Lock()
	defer p.mux.Unlock()
	delete(p.programs, programHash(prog.Name, prog.Job, prog.TaskId))
}

func (p *programMap) toArray() []Program {
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
