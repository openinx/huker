package supervisor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

type SupervisorCli struct {
	ServerAddr string
}

func NewSupervisorCli(serverAddr string) *SupervisorCli {
	return &SupervisorCli{
		ServerAddr: serverAddr,
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

func (s *SupervisorCli) Bootstrap(p *Program) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	url := s.ServerAddr + "/api/programs"
	_, err2 := request("POST", url, bytes.NewBuffer(data))
	return err2
}

func (s *SupervisorCli) Show(name, job string, taskId int) (*Program, error) {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d", s.ServerAddr, name, job, taskId)
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

func (s *SupervisorCli) Start(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/start", s.ServerAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}

func (s *SupervisorCli) Cleanup(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d", s.ServerAddr, name, job, taskId)
	_, err := request("DELETE", url, nil)
	return err
}

func (s *SupervisorCli) RollingUpdate(p *Program) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	url := s.ServerAddr + "/api/programs/rolling_update"
	_, err2 := request("POST", url, bytes.NewBuffer(data))
	return err2
}

func (s *SupervisorCli) Restart(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/restart", s.ServerAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}

func (s *SupervisorCli) Stop(name, job string, taskId int) error {
	url := fmt.Sprintf("%s/api/programs/%s/%s/%d/stop", s.ServerAddr, name, job, taskId)
	_, err := request("PUT", url, nil)
	return err
}

func (s *SupervisorCli) ListTasks() ([]*Program, error) {
	url := fmt.Sprintf("%s/api/programs", s.ServerAddr)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s", data)
	}
	var programs []*Program
	if err := json.Unmarshal(data, &programs); err != nil {
		return nil, err
	}
	return programs, nil
}

func (s *SupervisorCli) GetTask(name, job string, taskId int) (*Program, error) {
	programs, err := s.ListTasks()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(programs); i++ {
		if programs[i].Name == name && programs[i].Job == job && programs[i].TaskId == taskId {
			return programs[i], nil
		}
	}
	return nil, fmt.Errorf("Task does not found")
}
