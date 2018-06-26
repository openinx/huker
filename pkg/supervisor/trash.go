package supervisor

import (
	"github.com/qiniu/log"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"
)

const (
	patternTrashDir = "\\.trash\\.([a-zA-Z0-9_\\-]+)\\.([0-9]+)\\.([0-9]+)"
)

type TrashCleaner struct {
	rootDir string
	ttlSec  int
}

func NewTrashCleaner(rootDir string, ttlSec int) *TrashCleaner {
	return &TrashCleaner{rootDir: rootDir, ttlSec: ttlSec}
}

// Return the timestamp of trash path, return -1 if path mismatch the patternTrashDir.
func trashDirTimestamp(path string) int {
	re := regexp.MustCompile(patternTrashDir)
	if re.MatchString(path) {
		match := re.FindStringSubmatch(path)
		if len(match) != 4 {
			log.Warnf("Trash directory format matched, but match length is not 4, is: %d", len(match))
			return -1
		}
		timestamp, err := strconv.Atoi(match[3])
		if err != nil {
			log.Warnf("Trash directory format matched, but timestamp is %s, not a int", match[3])
			return -1
		}
		return timestamp
	}
	return -1
}

func isTrashDir(path string) bool {
	return trashDirTimestamp(path) >= 0
}

type visitor func(fullPath string) error

func (t *TrashCleaner) doExecute(expireDirFunc visitor) error {
	files, err := ioutil.ReadDir(t.rootDir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			dir := path.Join(t.rootDir, f.Name())
			subDirs, err := ioutil.ReadDir(dir)
			if err != nil {
				log.Errorf("Failed to list the sub-directories for dir: %s, %v", dir, err)
				continue
			}
			for _, subDir := range subDirs {
				if timestamp := trashDirTimestamp(subDir.Name()); timestamp >= 0 && subDir.IsDir() {
					expireTime := int64(timestamp + t.ttlSec)
					if expireTime <= time.Now().Unix() {
						fullPath := path.Join(dir, subDir.Name())
						if err := expireDirFunc(fullPath); err != nil {
							log.Errorf("Failed to remove the expired trash directory: %s, %v", fullPath, err)
							continue
						}
					}
				}
			}
		}
	}
	return nil
}

func (t *TrashCleaner) CheckAndClean() error {
	return t.doExecute(os.RemoveAll)
}
