package thirdparts

import (
	"github.com/openinx/huker/pkg/utils"
	"strings"
	"time"
)

type NodeMetricFetcher struct {
	url  string
	host string
}

func NewNodeMetricFetcher(url string, host string) (*NodeMetricFetcher, error) {
	return &NodeMetricFetcher{url: url, host: host}, nil
}

func formatMetric(metric string, timestamp int64, value float64, tags map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"metric":    metric,
		"timestamp": timestamp,
		"value":     value,
		"tags":      tags,
	}
}

func (f *NodeMetricFetcher) Pull() (interface{}, error) {
	jsonMap, err := utils.HttpGetJSON(f.url)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	now := time.Now().Unix()

	tags := map[string]interface{}{
		"host": f.host,
	}

	// CPU: Usage Percent
	result = append(result, formatMetric("sys.cpu.percent", now, jsonMap["CpuPercents"].(float64), tags))

	// Load1
	loadMap := jsonMap["Load"].(map[string]interface{})
	result = append(result, formatMetric("sys.cpu.load1", now, loadMap["load1"].(float64), tags))

	// Load5
	result = append(result, formatMetric("sys.cpu.load5", now, loadMap["load5"].(float64), tags))

	// Load15
	result = append(result, formatMetric("sys.cpu.load15", now, loadMap["load15"].(float64), tags))

	// sys.mem.free
	mem := jsonMap["memory"].(map[string]interface{})
	result = append(result, formatMetric("sys.mem.free", now, mem["free"].(float64), tags))

	// sys.mem.avail
	result = append(result, formatMetric("sys.mem.available", now, mem["available"].(float64), tags))

	// sys.mem.total
	result = append(result, formatMetric("sys.mem.total", now, mem["total"].(float64), tags))

	// sys.mem.used
	result = append(result, formatMetric("sys.mem.used", now, mem["used"].(float64), tags))

	// sys.mem.used_percent
	result = append(result, formatMetric("sys.mem.usedPercent", now, mem["usedPercent"].(float64), tags))

	// sys.net.<key>.<interface-name>
	interfaces := jsonMap["Network"].([]interface{})
	for i := 0; i < len(interfaces); i++ {
		ifEntry := interfaces[i].(map[string]interface{})
		ifName := ifEntry["name"].(string)

		for _, key := range []string{
			"bytesRecv",
			"bytesSent",
			"packetsRecv",
			"packetsSent",
			"dropin",
			"dropout",
			"errin",
			"errout",
		} {
			result = append(result, formatMetric("sys.net."+key, now, ifEntry[key].(float64), map[string]interface{}{
				"host": f.host,
				"if":   ifName,
			}))
		}
	}

	// sys.disk
	duMap := jsonMap["DiskUsage"].(map[string]interface{})
	for diskKey := range duMap {
		if strings.HasPrefix(diskKey, "usage:") {
			stat := duMap[diskKey].(map[string]interface{})
			disk := strings.TrimPrefix(diskKey, "usage:")
			for _, key := range []string{
				"free",
				"total",
				"used",
				"usedPercent",
			} {
				result = append(result, formatMetric("sys.disk."+key, now, stat[key].(float64), map[string]interface{}{
					"host": f.host,
					"disk": disk,
				}))
			}
		}
	}

	return result, nil
}
