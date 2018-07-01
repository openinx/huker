package grafana

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openinx/huker/pkg/core"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type GrafanaSyncer struct {
	grafanaAddr       string
	apiKey            string
	dataSourceKey     string
	networkInterfaces []string
	diskDevices       []string
}

func NewGrafanaSyncer(grafanaAddr string, apiKey string, dataSourceKey string,
	networkInterfaces []string, diskDevices []string) *GrafanaSyncer {
	return &GrafanaSyncer{
		grafanaAddr:       grafanaAddr,
		apiKey:            apiKey,
		dataSourceKey:     dataSourceKey,
		networkInterfaces: networkInterfaces,
		diskDevices:       diskDevices,
	}
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

func (g *GrafanaSyncer) postDashboard(uid, title string, jsonMap map[string]interface{}) error {
	jsonMap["title"] = title
	jsonMap["uid"] = uid
	jsonMap["id"] = nil // unset the id intentionally
	dashMap := map[string]interface{}{
		"overwrite": true,
		"dashboard": jsonMap,
	}
	data, err := json.Marshal(dashMap)
	if err != nil {
		return err
	}
	_, respErr := g.request("POST", "/api/dashboards/db", data)
	return respErr
}

func (g *GrafanaSyncer) GetDashboard(uid string) ([]byte, error) {
	return g.request("GET", "/api/dashboards/uid/"+uid, nil)
}

func (g *GrafanaSyncer) CreateHostDashboard(hostname string) error {
	uid := "host-" + strings.Replace(hostname, ".", "-", -1)
	return g.createHostsDashboard(uid, uid, []string{hostname})
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(m)
	newMap := make(map[string]interface{})
	json.Unmarshal(data, &newMap)
	return newMap
}

// Each target represent a curve in chart. if multiple targets in a panel, then
// it will has multiple curves shown in the same chart. Here we try to replace
// tags of one targetMap in the targetsMap to generate a new target.
func generateNewTargetMap(targetMaps []interface{}, newTags map[string]string) map[string]interface{} {
	for _, targetMap := range targetMaps {
		t := targetMap.(map[string]interface{})
		newTarget := copyMap(t)
		newTarget["tags"] = newTags
		// Only need to handle one targetMap, because we will map to all hosts in upper layer.
		return newTarget
	}
	panic("Found no target map")
}

func (g *GrafanaSyncer) createHostsDashboard(title, uid string, hostNames []string) error {
	jsonMap, err := loadGrafanaJsonMap("host.json")
	if err != nil {
		return err
	}
	// Generate the new panel maps.
	panelMaps := jsonMap["panels"].([]interface{})
	for _, panelMap := range panelMaps {
		p := panelMap.(map[string]interface{})
		targetMaps := p["targets"].([]interface{})
		p["datasource"] = g.dataSourceKey
		mKey := p["title"].(string)
		if strings.HasPrefix(mKey, "sys.disk.") {
			var newTargets []interface{}
			for _, dd := range g.diskDevices {
				for _, hostName := range hostNames {
					target := generateNewTargetMap(targetMaps, map[string]string{"host": hostName, "disk": dd})
					newTargets = append(newTargets, target)
				}
			}
			p["targets"] = newTargets
		} else if strings.HasPrefix(mKey, "sys.net.") {
			var newTargets []interface{}
			for _, nif := range g.networkInterfaces {
				for _, hostName := range hostNames {
					target := generateNewTargetMap(targetMaps, map[string]string{"host": hostName, "if": nif})
					newTargets = append(newTargets, target)
				}
			}
			p["targets"] = newTargets
		} else {
			newTargets := make([]interface{}, len(hostNames))
			for hostIdx := range hostNames {
				newTargets[hostIdx] = generateNewTargetMap(targetMaps, map[string]string{"host": hostNames[hostIdx]})
			}
			p["targets"] = newTargets
		}
	}
	return g.postDashboard(uid, title, jsonMap)
}

func (g *GrafanaSyncer) CreateNodesDashboard(cluster string, hostNames []string) error {
	uid := "nodes-" + cluster
	return g.createHostsDashboard(uid, uid, hostNames)
}

func (g *GrafanaSyncer) CreateJvmGcDashboard(cluster string, job string, task int) error {
	jsonMap, err := loadGrafanaJsonMap("task_gc.json")
	if err != nil {
		return err
	}
	panelMaps := jsonMap["panels"].([]interface{})
	for _, panelMap := range panelMaps {
		p := panelMap.(map[string]interface{})
		targetMaps := p["targets"].([]interface{})
		p["datasource"] = g.dataSourceKey

		for _, targetMap := range targetMaps {
			t := targetMap.(map[string]interface{})
			t["tags"] = map[string]string{
				"cluster": cluster,
				"job":     job,
				"task":    strconv.Itoa(task),
			}
		}
	}
	label := fmt.Sprintf("jvm-%s-%s-%d", cluster, job, task)
	return g.postDashboard(label, label, jsonMap)
}

func (g *GrafanaSyncer) CreateHDFSDashboard(c *core.Cluster) error {
	jsonMap, err := loadGrafanaJsonMap("hdfs.json")
	if err != nil {
		return err
	}
	panelMaps := jsonMap["panels"].([]interface{})
	for _, panelMap := range panelMaps {
		p := panelMap.(map[string]interface{})
		titleName := p["title"].(string)
		targetMaps := p["targets"].([]interface{})
		p["datasource"] = g.dataSourceKey

		var targets []interface{}
		if strings.HasPrefix(titleName, "hdfs.namenode") {
			newTarget := generateNewTargetMap(targetMaps, map[string]string{
				"cluster": c.ClusterName,
				"job":     "namenode",
			})
			targets = append(targets, newTarget)
		} else if strings.HasPrefix(titleName, "hdfs.datanode") {
			for _, host := range c.Jobs["datanode"].Hosts {
				newTarget := generateNewTargetMap(targetMaps, map[string]string{
					"cluster":     c.ClusterName,
					"job":         "datanode",
					"hostAndPort": fmt.Sprintf("%s-%d", host.Hostname, host.BasePort+1),
				})
				targets = append(targets, newTarget)
			}
		}
		p["targets"] = targets
	}
	label := fmt.Sprintf("cluster-hdfs-%s", c.ClusterName)
	return g.postDashboard(label, label, jsonMap)
}

func (g *GrafanaSyncer) CreateZookeeperDashboard(c *core.Cluster) error {
	jsonMap, err := loadGrafanaJsonMap("zookeeper.json")
	if err != nil {
		return err
	}

	panelMaps := jsonMap["panels"].([]interface{})
	for _, panelMap := range panelMaps {
		p := panelMap.(map[string]interface{})
		targetMaps := p["targets"].([]interface{})
		p["datasource"] = g.dataSourceKey

		var targets []interface{}
		for _, host := range c.Jobs["zkServer"].Hosts {
			newTarget := generateNewTargetMap(targetMaps, map[string]string{
				"cluster":     c.ClusterName,
				"job":         "zkServer",
				"hostAndPort": fmt.Sprintf("%s-%d", host.Hostname, host.BasePort),
			})
			targets = append(targets, newTarget)
		}
		p["targets"] = targets
	}
	label := fmt.Sprintf("cluster-zookeeper-%s", c.ClusterName)
	return g.postDashboard(label, label, jsonMap)
}

func (g *GrafanaSyncer) CreateHBaseDashboard(c *core.Cluster) error {
	jsonMap, err := loadGrafanaJsonMap("hbase.json")
	if err != nil {
		return err
	}
	panelMaps := jsonMap["panels"].([]interface{})
	for _, panelMap := range panelMaps {
		p := panelMap.(map[string]interface{})
		targetMaps := p["targets"].([]interface{})
		p["datasource"] = g.dataSourceKey

		var targets []interface{}
		for _, host := range c.Jobs["regionserver"].Hosts {
			newTarget := generateNewTargetMap(targetMaps, map[string]string{
				"cluster":     c.ClusterName,
				"job":         "regionserver",
				"hostAndPort": fmt.Sprintf("%s-%d", host.Hostname, host.BasePort+1),
			})
			targets = append(targets, newTarget)
		}
		p["targets"] = targets
	}
	label := fmt.Sprintf("cluster-hbase-%s", c.ClusterName)
	return g.postDashboard(label, label, jsonMap)
}
