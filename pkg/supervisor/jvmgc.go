package supervisor

import (
	"bytes"
	"fmt"
	"github.com/qiniu/log"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

func parseJstatStdout(stdout bytes.Buffer) (map[string]float64, error) {
	lines := strings.Split(stdout.String(), "\n")
	if len(lines) != 3 {
		return nil, fmt.Errorf("Line number of `jstat -gc` stdout should be 3.")
	}
	var values []float64
	for _, data := range strings.Split(lines[1], " ") {
		if len(data) != 0 {
			val, err := strconv.ParseFloat(data, 32)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
	}
	if len(values) != 17 {
		return nil, fmt.Errorf("Column number of `jstat -gc` should be 17.")
	}
	gcData := make(map[string]float64)
	for idx, gcKey := range []string{
		"survivor0.capacity", // KB
		"survivor1.capacity",
		"survivor0.usage",
		"survivor1.usage",
		"eden.capacity",
		"eden.usage",
		"old.space.capacity",
		"old.space.usage",
		"metaspace.capacity",
		"metaspace.usage",
		"compressed.class.space.capacity",
		"compressed.class.space.usage",
		"young.gc.count",
		"young.gc.time", // second
		"full.gc.count",
		"full.gc.time",  // second
		"total.gc.time", // second
	} {
		gcData[gcKey] = values[idx]
	}
	return gcData, nil
}

func JstatGC(javaHome string, pid int) (map[string]float64, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(path.Join(javaHome, "bin", "jstat"), "-gc", strconv.Itoa(pid))
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		log.Warnf("`jstat -gc %d` error: %s, stdout: [%s], stderr: [%s]", pid, err, stdout.String(), stderr.String())
		return nil, err
	}
	return parseJstatStdout(stdout)
}
