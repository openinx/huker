package pkg

import (
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"path"
	"reflect"
	"testing"
)

const (
	typeInt   = 0
	typeStr   = 1
	typeURL   = 2
	typeSlice = 3
)

func TestHukerConfig(t *testing.T) {
	confFile := path.Join(utils.GetHukerSourceDir(), "conf", "huker.yaml")
	h, err := NewHukerConfig(confFile)
	if err != nil {
		t.Fatal(err)
	}

	var testCases = []struct {
		key          string
		val          interface{}
		expectedType int
	}{
		{HukerPkgSrvHttpAddress, "http://127.0.0.1:4000", typeURL},
		{HukerDashboardHttpAddress, "http://127.0.0.1:8001", typeURL},
		{HukerOpenTSDBHttpAddress, "http://127.0.0.1:51001", typeURL},
		{HukerGrafanaHttpAddress, "http://127.0.0.1:3000", typeURL},
		{HukerGrafanaAPIKey, "Bearer eyJrIjoiSW9JelRJd2xSN3c2ZGZEMVBuUXdhbFJJQ0txR2pqR2wiLCJuIjoiaHVrZXIiLCJpZCI6MX0=", typeStr},
		{HukerGrafanaDataSource, "test-opentsdb", typeStr},
		{HukerCollectorWorkerSize, 10, typeInt},
		{HukerCollectorSyncDashboardSeconds, 86400, typeInt},
		{HukerCollectorCollectSeconds, 5, typeInt},
		{HukerCollectorNetworkInterfaces, []string{"lo0", "en0"}, typeSlice},
		{HukerCollectorDiskDevices, []string{"/dev/disk1", "/dev/disk2s2"}, typeSlice},
		{HukerSupervisorPort, 9001, typeInt},
	}

	for i := range testCases {
		if testCases[i].expectedType == typeInt {
			expected := (testCases[i].val).(int)
			actual := h.GetInt(testCases[i].key)
			if actual != actual {
				t.Errorf("Case#%d: Int value of key:%s mismatch, expected: %d, actual: %d", i, testCases[i].key, expected, actual)
			}
		} else if testCases[i].expectedType == typeURL {
			expected := testCases[i].val.(string)
			u, err := h.GetURL(testCases[i].key)
			if err != nil {
				log.Errorf("Case#%d: Failed to parse the url: %s, %v", i, expected, err)
				continue
			}
			if expected != u.String() {
				t.Errorf("Case#%d: Value of key: %s mismatch, expected: %s, actual: %s", i, testCases[i].key, expected, u.String())
			}
		} else if testCases[i].expectedType == typeStr {
			expected := testCases[i].val.(string)
			actual := h.Get(testCases[i].key)
			if expected != actual {
				t.Errorf("Case#%d: Value of key: %s mismatch, expected: %s, actual: %s", i, testCases[i].key, expected, actual)
			}
		} else if testCases[i].expectedType == typeSlice {
			expected := testCases[i].val.([]string)
			actual := h.GetSlice(testCases[i].key)
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("Case#%d: value of key: %s mismatch, expected: %v, actual: %v", i, testCases[i].key, expected, actual)
			}
		}
	}
}
