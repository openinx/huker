package huker

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/qiniu/log"
	"io"
	"os"
	"strconv"
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

func calcFileMD5Sum(fName string) (string, error) {
	f, err := os.Open(fName)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hashReader := md5.New()
	if _, err := io.Copy(hashReader, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hashReader.Sum(nil)), nil
}

func ReadEnvStrValue(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func ReadEnvIntValue(key string, defaultVal int) int {
	val := ReadEnvStrValue(key, string(defaultVal))
	if val == "" {
		return defaultVal
	}
	if valInt, err := strconv.Atoi(val); err != nil {
		return defaultVal
	} else {
		return valInt
	}
}
