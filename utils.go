package huker

import (
	"github.com/qiniu/log"
	"os"
	"syscall"
)

func isProcessOK(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Infof("Failed to find process[pid: %d]: %v", pid, err)
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		log.Infof("process.Signal on pid %d returned: %v", pid, err)
		return false
	}
	return true
}
