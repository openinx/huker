package grafana

import (
	"encoding/json"
	"github.com/openinx/huker/pkg/utils"
	"io/ioutil"
	"path"
)

func loadGrafanaJsonMap(jsonFile string) (map[string]interface{}, error) {
	hukerDir := utils.GetHukerDir()
	data, err := ioutil.ReadFile(path.Join(hukerDir, "grafana", jsonFile))
	if err != nil {
		return nil, err
	}
	jsonMap := make(map[string]interface{})
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return nil, err
	}
	return jsonMap, nil
}
