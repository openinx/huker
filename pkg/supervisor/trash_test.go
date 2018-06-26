package supervisor

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"
)

func TestIsTrashDir(t *testing.T) {
	var testCases = []struct {
		dir        string
		timestamp  int
		isTrashDir bool
	}{
		{".trash.master.0.1525869093", 1525869093, true},
		{".trash.master.0.1525869093", 1525869093, true},
		{".trash.master.0.0", 0, true},
		{"master.0", -1, false},
		{".test.master.0.1234567", -1, false},
	}

	for caseId, cas := range testCases {
		if isTrashDir(cas.dir) != cas.isTrashDir {
			t.Errorf("Case#%d isTrashDir mismatch: %v != %v", caseId, isTrashDir(cas.dir), cas.isTrashDir)
		}
		if trashDirTimestamp(cas.dir) != cas.timestamp {
			t.Errorf("Case#%d timestamp mismatch: %d != %d", caseId, trashDirTimestamp(cas.dir), cas.timestamp)
		}
	}
}

func TestDoExecute(t *testing.T) {
	rootDir := fmt.Sprintf("/tmp/hukerTrashTest.%d", time.Now().Unix())
	// defer os.RemoveAll(rootDir)
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, i := range []string{"a", "b", "c"} {
		dir := path.Join(rootDir, i)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(path.Join(dir, ".trash."+i+".0.1525869093"), 0755); err != nil {
			t.Fatal(err)
		}
		if f, err := os.Create(path.Join(dir, ".trash.file.0.1525869093")); err == nil {
			f.Close()
		}
	}

	var pathList []string
	tc := NewTrashCleaner(rootDir, 86400)
	err := tc.doExecute(func(path string) error {
		pathList = append(pathList, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pathList) != 3 {
		t.Errorf("Path size is not 3, is %d", len(pathList))
	}
	expectedDirs := []string{
		path.Join(rootDir, "a", ".trash.a.0.1525869093"),
		path.Join(rootDir, "b", ".trash.b.0.1525869093"),
		path.Join(rootDir, "c", ".trash.c.0.1525869093"),
	}
	for i := 0; i < len(pathList); i++ {
		if pathList[i] != expectedDirs[i] {
			t.Errorf("Directory mismatch, %s != %s", pathList[i], expectedDirs[i])
		}
	}
}
