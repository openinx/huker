package pkg

import (
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"path"
	"testing"
)

func TestHukerConfig(t *testing.T) {
	confFile := path.Join(utils.GetHukerDir(), "conf", "huker.yaml")
	h, err := NewHukerConfig(confFile)
	if err != nil {
		t.Fatal(err)
	}

	var testCases = []struct {
		key   string
		val   interface{}
		isInt bool
		isURL bool
	}{
		{HukerPkgSrvHttpAddress, "http://127.0.0.1:4000", false, true},
		{HukerDashboardHttpAddress, "http://127.0.0.1:8001", false, true},
		{HukerOpenTSDBHttpAddress, "http://127.0.0.1:51001", false, true},
		{HukerGrafanaHttpAddress, "http://127.0.0.1:3000", false, true},
		{HukerGrafanaAPIKey, "Bearer eyJrIjoiSW9JelRJd2xSN3c2ZGZEMVBuUXdhbFJJQ0txR2pqR2wiLCJuIjoiaHVrZXIiLCJpZCI6MX0=", false, false},
		{HukerGrafanaDataSource, "test-opentsdb", false, false},
		{HukerCollectorWorkerSize, 10, true, false},
		{HukerSupervisorPort, 9001, true, false},
	}

	for i := range testCases {
		if testCases[i].isInt {
			expected := (testCases[i].val).(int)
			actual := h.GetInt(testCases[i].key)
			if actual != actual {
				t.Errorf("Case#%d: Int value of key:%s mismatch, expected: %d, actual: %d", i, testCases[i].key, expected, actual)
			}
		} else if testCases[i].isURL {
			expected := testCases[i].val.(string)
			u, err := h.GetURL(testCases[i].key)
			if err != nil {
				log.Errorf("Case#%d: Failed to parse the url: %s, %v", i, expected, err)
				continue
			}
			if expected != u.String() {
				t.Errorf("Case#%d: Value of key: %s mismatch, expected: %d, actual: %d", i, testCases[i].key, expected, u.String())
			}
		} else {
			expected := testCases[i].val.(string)
			actual := h.Get(testCases[i].key)
			if expected != actual {
				t.Errorf("Case#%d: Value of key: %s mismatch, expected: %d, actual: %d", i, testCases[i].key, expected, actual)
			}
		}
	}
}
