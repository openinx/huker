package huker

import "testing"

func TestHost(t *testing.T) {
	type TestCase struct {
		hostKey     string
		result      bool
		httpAddress string
		key         string
	}

	testCases := []TestCase{
		{"192.168.0.1:2001/id=0/base_port=9001", true, "http://192.168.0.1:2001", "192.168.0.1:2001/id=0"},
		{"192.168.0.1:abc/id=0/base_port=9001", false, "", ""},
		{"www.example.com/id=0/base_port=9001", false, "", ""},
		{"www.example.com:9001/id=0/base_port=9001", true, "http://www.example.com:9001", "www.example.com:9001/id=0"},
		{"www.example.com:9001", true, "http://www.example.com:9001", "www.example.com:9001/id=0"},
		{"www.example.com:9001/id=a/base_port=9001", false, "", ""},
		{"www.example.com:9001/id=1/base_port=-9001", false, "", ""},
		{"www.example.com:9001/id=-1/base_port=9001", false, "", ""},
		{"localhost/id=1/base_port=9001", false, "", ""},
		{"localhost:9001/id=1/base_port=9001/abc", false, "", ""},
		{"localhost:9001/id=1/base_port=9001/a=b", true, "http://localhost:9001", "localhost:9001/id=1"},
		{"localhost:9001/id=1/base_port=9001/a=b/c=d", true, "http://localhost:9001", "localhost:9001/id=1"},
		{"localhost:9001/id=1/base_port=9001/cpu=16/mem=2801M", true, "http://localhost:9001", "localhost:9001/id=1"},
	}

	for idx, cas := range testCases {
		host, err := NewHost(testCases[idx].hostKey)
		if cas.result {
			if err != nil {
				t.Errorf("Case#%d failed, key: %s, err: %v", idx, testCases[idx].hostKey, err)
			}
			if cas.httpAddress != host.toHttpAddress() {
				t.Errorf("Case#%d failed, %s != %s", idx, cas.httpAddress, host.toHttpAddress())
			}
			if cas.key != host.ToKey() {
				t.Errorf("Case#%d failed, %s != %s", idx, cas.key, host.ToKey())
			}
		}
		if !cas.result {
			if err == nil {
				t.Errorf("Case#%d should be failed, key: %s", idx, testCases[idx].hostKey)
			}
		}
	}
}
