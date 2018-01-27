package huker

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/qiniu/log"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"syscall"
)

func isProcessOK(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Debugf("Failed to find process[pid: %d]: %v", pid, err)
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		log.Debugf("process.Signal on pid %d returned: %v", pid, err)
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

// Download from fileHttpAddr to local file named localFileName.
func WebGetToLocal(fileHttpAddr string, localFileName string) error {
	resp, err := http.Get(fileHttpAddr)
	if err != nil {
		log.Errorf("Downloading file failed. file: %s, err: %s", fileHttpAddr, err.Error())
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Errorf("Downloading file failed. file: %s, err: %s", fileHttpAddr, resp.Status)
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(data))
	}
	out, err := os.Create(localFileName)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

// Run a bash command, the env will set be to default the env of current process if pass nil to env.
func RunCommand(name string, env []string, args ...string) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	fullCmd := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Environment variables:\n%s", strings.Join(cmd.Env, "\n"))
		log.Errorf("Run command failed. [cmd: %s], [stdout: %s], [stderr: %s]",
			fullCmd, stdout.String(), stderr.String())
		return err
	}
	return nil
}

// Read string value of env for the specific key, use default value if not key found in env.
func ReadEnvStrValue(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// Read int value of env for the specific key, use default value if not key found in env.
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

// True to indicate that the slice contains the s.
func StringSliceContains(slice []string, s string) bool {
	for i := range slice {
		if slice[i] == s {
			return true
		}
	}
	return false
}

// True to indicate that x is a string type.
func IsStringType(x interface{}) bool {
	return x != nil && reflect.TypeOf(x).Kind() == reflect.String
}

// True to indicate that x is a int type
func IsIntegerType(x interface{}) bool {
	return x != nil && reflect.TypeOf(x).Kind() == reflect.Int
}

// True to indicate that x is a map type
func IsMapType(x interface{}) bool {
	return x != nil && reflect.TypeOf(x).Kind() == reflect.Map
}

// True to indicate that x is a slice type
func IsSliceType(x interface{}) bool {
	return x != nil && reflect.TypeOf(x).Kind() == reflect.Slice
}

// True to indicate that x is array type
func IsArrayType(x interface{}) bool {
	return x != nil && reflect.TypeOf(x).Kind() == reflect.Array
}

// Merge map m2 to map m1, if a key exist in both map m1 and map m2, then use the value of m1.
// The returned map is the map m1.
func MergeMap(m1 map[interface{}]interface{}, m2 map[interface{}]interface{}) map[interface{}]interface{} {
	if !IsMapType(m2) {
		return m1
	}
	if !IsMapType(m1) {
		return m2
	}

	for key := range m2 {
		value := m2[key]
		if value == nil {
			continue
		}
		if m1[key] == nil {
			m1[key] = value
			continue
		}
		if IsSliceType(value) || IsArrayType(value) {
			if !IsSliceType(m1[key]) && !IsArrayType(m1[key]) {
				panic("Type mismatch")
			}
			a1 := m1[key].([]interface{})
			a2 := m2[key].([]interface{})
			var a3 []interface{}
			for i := range a1 {
				a3 = append(a3, a1[i])
			}
			for i := range a2 {
				exist := false
				for j := range a1 {
					if a1[j] == a2[i] {
						exist = true
					}
				}
				if exist {
					continue
				}
				a3 = append(a3, a2[i])
			}
			m1[key] = a3
		} else if IsMapType(value) {
			if !IsMapType(m1[key]) {
				panic("Type mismatch")
			}
			m1[key] = MergeMap(m1[key].(map[interface{}]interface{}), m2[key].(map[interface{}]interface{}))
		}
	}

	return m1
}
