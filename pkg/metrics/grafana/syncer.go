package grafana

import (
	"bytes"
	"errors"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"text/template"
)

type GrafanaSyncer struct {
	grafanaAddr   string
	apiKey        string
	dataSourceKey string
}

func NewGrafanaSyncer(grafanaAddr string, apiKey string, dataSourceKey string) *GrafanaSyncer {
	return &GrafanaSyncer{
		grafanaAddr:   grafanaAddr,
		apiKey:        apiKey,
		dataSourceKey: dataSourceKey,
	}
}

func RenderJsonTemplate(args map[string]string, jsonFile string) ([]byte, error) {
	hukerDir := utils.GetHukerDir()
	data, err := ioutil.ReadFile(path.Join(hukerDir, jsonFile))
	if err != nil {
		log.Errorf("Failed to read file: %s, %v", jsonFile, err)
		return nil, err
	}

	t := template.New(jsonFile + "-template")
	t, err = t.Parse(string(data))
	if err != nil {
		log.Errorf("Failed to parse the template file, %v", err)
		return nil, err
	}

	var buf bytes.Buffer
	if err = t.Execute(&buf, args); err != nil {
		log.Errorf("Failed to render the template file: %s, %v ", jsonFile, err.Error())
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *GrafanaSyncer) request(method, resource string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	fullURL := g.grafanaAddr + resource
	if body == nil || len(body) <= 0 {
		req, err = http.NewRequest(method, fullURL, nil)
	} else {
		req, err = http.NewRequest(method, fullURL, bytes.NewBuffer(body))
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", g.apiKey)

	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respData, _ := ioutil.ReadAll(resp.Body)
		return nil, errors.New(string(respData))
	}
	return ioutil.ReadAll(resp.Body)
}

func (g *GrafanaSyncer) GetDashboard(uid string) ([]byte, error) {
	return g.request("GET", "/api/dashboards/uid/"+uid, nil)
}

func (g *GrafanaSyncer) CreateHostDashboardIfNotExist(hostname string) error {
	data, err := RenderJsonTemplate(map[string]string{
		"HostName":   hostname,
		"DataSource": g.dataSourceKey,
		"Tittle":     "host-" + strings.Replace(hostname, ".", "-", -1),
		"Uid":        "host-" + strings.Replace(hostname, ".", "-", -1),
	}, "grafana/host.json")
	if err != nil {
		return err
	}
	_, respErr := g.request("POST", "/api/dashboards/db", data)
	return respErr
}
