package huker

import (
	"os"
	"strconv"
	"testing"
)

func TestIsProcessOK(t *testing.T) {

	if isProcessOK(64236) {
		t.Errorf("Process %d is stopped actually.", 64236)
	}
}

func TestReadEnvStrValue(t *testing.T) {
	os.Setenv("hello", "world")
	os.Setenv("intEnv", strconv.Itoa(100))

	type StrNode struct {
		key          string
		defaultValue string
		getValue     string
	}

	case1 := []StrNode{
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

	type IntNode struct {
		key          string
		defaultValue int
		getValue     int
	}

	case2 := []IntNode{
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
