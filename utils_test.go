package huker

import "testing"

func TestIsProcessOK(t *testing.T) {

	if isProcessOK(64236) {
		t.Errorf("Process %d is stopped actually.", 64236)
	}
}
