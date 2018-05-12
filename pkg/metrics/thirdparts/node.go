package thirdparts

import (
	"github.com/influxdata/influxdb/client/v2"
	"github.com/openinx/huker/pkg/utils"
	"strings"
)

type NodeMetricFetcher struct {
	url  string
	host string
}

func NewNodeMetricFetcher(url string, host string) (*NodeMetricFetcher, error) {
	return &NodeMetricFetcher{url: url, host: host}, nil
}

func (f *NodeMetricFetcher) Pull(conf client.BatchPointsConfig) (client.BatchPoints, error) {
	bp, err := client.NewBatchPoints(conf)
	jsonMap, err := utils.HttpGetJSON(f.url)
	if err != nil {
		return bp, err
	}

	if err != nil {
		return bp, err
	}

	// CPU: Usage Percent / Load
	load := jsonMap["Load"].(map[string]interface{})
	p, err := client.NewPoint("node_cpu", map[string]string{
		"host": f.host,
		"key":  "cpu",
	}, map[string]interface{}{
		"cpu_usage_percent": jsonMap["CpuPercents"].(float64),
		"load1":             load["load1"].(float64),
		"load5":             load["load5"].(float64),
		"load15":            load["load15"].(float64),
	})
	if err != nil {
		return bp, err
	}
	bp.AddPoint(p)

	// Memory
	mem := jsonMap["memory"].(map[string]interface{})
	p, err = client.NewPoint("node_memory", map[string]string{
		"host": f.host,
		"key":  "memory",
	}, map[string]interface{}{
		"available":   mem["available"].(float64),
		"free":        mem["free"].(float64),
		"total":       mem["total"].(float64),
		"used":        mem["used"].(float64),
		"usedPercent": mem["usedPercent"].(float64),
	})
	if err != nil {
		return bp, err
	}
	bp.AddPoint(p)

	// Network
	interfaces := jsonMap["Network"].([]interface{})
	for i := 0; i < len(interfaces); i++ {
		interf := interfaces[i].(map[string]interface{})
		p, err = client.NewPoint("node_network", map[string]string{
			"host":      f.host,
			"interface": interf["name"].(string),
		}, map[string]interface{}{
			"bytesRecv":   interf["bytesRecv"].(float64),
			"bytesSent":   interf["bytesSent"].(float64),
			"packetsRecv": interf["packetsRecv"].(float64),
			"packetsSent": interf["packetsSent"].(float64),
			"dropin":      interf["dropin"].(float64),
			"dropout":     interf["dropout"].(float64),
			"errin":       interf["errin"].(float64),
			"errout":      interf["errout"].(float64),
		})
		if err != nil {
			return bp, err
		}
		bp.AddPoint(p)
	}

	// Disk
	duMap := jsonMap["DiskUsage"].(map[string]interface{})
	for mountPoint := range duMap {
		if strings.HasPrefix(mountPoint, "usage:") {
			stat := duMap[mountPoint].(map[string]interface{})
			p, err = client.NewPoint("node_disk", map[string]string{
				"host":       f.host,
				"mountpoint": strings.TrimPrefix(mountPoint, "usage:"),
			}, map[string]interface{}{
				"free":        stat["free"].(float64),
				"total":       stat["total"].(float64),
				"used":        stat["used"].(float64),
				"usedPercent": stat["usedPercent"].(float64),
			})
			if err != nil {
				return bp, err
			}
			bp.AddPoint(p)
		}
	}

	return bp, nil
}
