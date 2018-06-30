package utils

import (
	"os"
	"strconv"
	"testing"
)

func TestIsProcessOK(t *testing.T) {

	if IsProcessOK(64236) {
		t.Errorf("Process %d is stopped actually.", 64236)
	}
}

func TestReadEnvStrValue(t *testing.T) {
	os.Setenv("hello", "world")
	os.Setenv("intEnv", strconv.Itoa(100))

	case1 := []struct {
		key          string
		defaultValue string
		getValue     string
	}{
		{"hello", "world0", "world"},
		{"hello", "world", "world"},
		{"foo", "bar", "bar"},
		{"foo", "", ""},
		{"intEnv", "101", "100"},
		{"intEnv", "100", "100"},
	}

	for index, cs := range case1 {
		if ReadEnvStrValue(cs.key, cs.defaultValue) != cs.getValue {
			t.Errorf("case #%d failed, %s != %s", index, ReadEnvStrValue(cs.key, cs.defaultValue), cs.getValue)
		}
	}

	case2 := []struct {
		key          string
		defaultValue int
		getValue     int
	}{
		{"hello", 9000, 9000},
		{"hello0", 234, 234},
		{"intEnv", 9001, 100},
		{"intEnv", 100, 100},
	}

	for index, cs := range case2 {
		if ReadEnvIntValue(cs.key, cs.defaultValue) != cs.getValue {
			t.Errorf("case #%d failed, %d != %d", index, ReadEnvIntValue(cs.key, cs.defaultValue), cs.getValue)
		}
	}
}

func TestFindJavaHome(t *testing.T) {
	javaHome, err := FindJavaHome("/home/huker/bin/java")
	if err != nil {
		t.Fatalf("Failed to find java home, %v", err)
	}
	if javaHome != "/home/huker" {
		t.Fatalf("Failed to parse java home from /home/huker/bin/java")
	}
}
