package huker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestSupervisorBootstrap(t *testing.T) {

	prog := &Program{
		Name: "tst-zk",
		Job:  "zookeeper.4",
		Bin:  "python",
		Args: []string{"-m", "SimpleHTTPServer"},
		Configs: map[string]string{
			"a": "b", "c": "d",
		},
		PkgAddress: "http://127.0.0.1:4000/zookeeper-3.4.11.tar.gz",
		PkgName:    "zookeeper-3.4.11.tar.gz",
		PkgMD5Sum:  "55aec6196ed9fa4c451cb5ae4a1f42d8",
	}

	data, err1 := json.Marshal(prog)
	if err1 != nil {
		t.Errorf("%v", err1)
	}

	buf := bytes.NewBuffer(data)

	fmt.Println(string(data))
	resp, err := http.Post("http://127.0.0.1:9001/api/programs", "type/json", buf)
	if err != nil {
		t.Errorf("%v", err)
	}
	data, _ = ioutil.ReadAll(resp.Body)

	respMap := make(map[string]interface{})
	json.Unmarshal(data, &respMap)

	fmt.Printf("%v\n", respMap["status"])
	fmt.Printf("%s\n", respMap["message"])
}
