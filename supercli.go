package huker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

type supervisorCli struct {
	serverAddr string
}

func newSupervisorCli(serverAddr string) *supervisorCli {
	return &supervisorCli{
		serverAddr: serverAddr,
	}
}

func handleResponse(resp *http.Response) ([]byte, error) {
	if resp.StatusCode >= 400 {
		data, _ := ioutil.ReadAll(resp.Body)
		return []byte{}, fmt.Errorf("%s, %s", resp.Status, data)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return data, err
	}
	if _, ok := m["message"]; !ok || (ok && m["message"].(string) == MESSAGE_SUCCESS) {
		return data, nil
	}
	return data, fmt.Errorf("%s", string(data))
}

func request(method, url string, body io.Reader) ([]byte, error) {
	req, err0 := http.NewRequest(method, url, body)
	if err0 != nil {
		return []byte{}, err0
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	return handleResponse(resp)
}

func (s *supervisorCli) bootstrap(p *Program) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	url := s.serverAddr + "/api/programs"
	_, err2 := request("POST", url, bytes.NewBuffer(data))
	return err2
}

func (s *supervisorCli) show(name, job string, taskId int) (*Program, error) {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d", s.serverAddr, name, job, taskId)
	data, err := request("GET", url, nil)
	if err != nil {
		return nil, err
	}
	p := &Program{}
	if err := json.Unmarshal(data, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *supervisorCli) start(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/start", s.serverAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}

func (s *supervisorCli) cleanup(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d", s.serverAddr, name, job, taskId)
	_, err := request("DELETE", url, nil)
	return err
}

func (s *supervisorCli) rollingUpdate(p *Program) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	url := s.serverAddr + "/api/programs/rolling_update"
	_, err2 := request("POST", url, bytes.NewBuffer(data))
	return err2
}

func (s *supervisorCli) restart(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/restart", s.serverAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}

func (s *supervisorCli) stop(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/stop", s.serverAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}
