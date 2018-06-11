package pkg

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"io/ioutil"
	"net/url"
)

const (
	// pkgsrv
	HukerPkgSrvHttpAddress = "huker.pkgsrv.http.address"

	// dashboard
	HukerDashboardHttpAddress = "huker.dashboard.http.address"

	// metric collector
	HukerOpenTSDBHttpAddress           = "huker.opentsdb.http.address"
	HukerGrafanaHttpAddress            = "huker.grafana.http.address"
	HukerGrafanaAPIKey                 = "huker.grafana.api.key"
	HukerGrafanaDataSource             = "huker.grafana.data.source"
	HukerCollectorWorkerSize           = "huker.collector.worker.size"
	HukerCollectorSyncDashboardSeconds = "huker.collector.sync.dashboard.seconds"
	HukerCollectorCollectSeconds       = "huker.collector.collect.seconds"

	// Supervisor agent
	HukerSupervisorPort = "huker.supervisor.http.port"
)

type HukerConfig struct {
	confFile string
	yamlMap  map[string]interface{}
}

func NewHukerConfig(confFile string) (*HukerConfig, error) {
	data, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, err
	}
	yamlMap := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &yamlMap); err != nil {
		return nil, err
	}

	return &HukerConfig{
		confFile: confFile,
		yamlMap:  yamlMap,
	}, nil
}

func (h *HukerConfig) GetInt(key string) int {
	if val, ok := h.yamlMap[key]; !ok {
		return 0
	} else {
		return val.(int)
	}
}

func (h *HukerConfig) Get(key string) string {
	if val, ok := h.yamlMap[key]; !ok {
		return ""
	} else {
		return val.(string)
	}
}

func (h *HukerConfig) GetURL(key string) (*url.URL, error) {
	if val, ok := h.yamlMap[key]; !ok {
		return nil, fmt.Errorf("key: %s not found.", key)
	} else {
		u, err := url.Parse(val.(string))
		if err != nil {
			return nil, err
		}
		return u, nil
	}
}
