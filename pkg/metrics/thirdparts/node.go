package thirdparts

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"strconv"
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

	// node.jvm.gc
	result = f.appendJavaMetrics(result, now, jsonMap)

	return result, nil
}

func parseClusterJobTask(label string) (string, string, int, error) {
	splits := strings.Split(label, "/")
	if len(splits) != 3 {
		return "", "", 0, fmt.Errorf("Splits should be 3, lable:%s", label)
	}
	var cluster, job string
	var task int
	if strings.HasPrefix(splits[0], "cluster=") {
		cluster = strings.TrimPrefix(splits[0], "cluster=")
	} else {
		return "", "", 0, fmt.Errorf("cluster not found, label:%s", label)
	}
	if strings.HasPrefix(splits[1], "job=") {
		job = strings.TrimPrefix(splits[1], "job=")
	} else {
		return "", "", 0, fmt.Errorf("job not found, label:%s", label)
	}
	if strings.HasPrefix(splits[2], "task_id=") {
		task, _ = strconv.Atoi(strings.TrimPrefix(splits[2], "task_id="))
	} else {
		return "", "", 0, fmt.Errorf("task_id not found, label:%s", label)
	}
	return cluster, job, task, nil
}

func (f *NodeMetricFetcher) appendJavaMetrics(result []map[string]interface{}, now int64, jsonMap map[string]interface{}) []map[string]interface{} {
	var javaMetrics map[string]interface{}
	if val, ok := jsonMap["JavaMetrics"]; !ok {
		return result
	} else {
		javaMetrics = val.(map[string]interface{})
	}
	for key, val := range javaMetrics {
		cluster, job, task, err := parseClusterJobTask(key)
		if err != nil {
			log.Errorf("Failed to parse the java metrics key: %s, error: %v", key, err)
			continue
		}
		metricsMap := val.(map[string]interface{})
		for mKey, mVal := range metricsMap {
			result = append(result, formatMetric("node.jvm.gc."+mKey, now, mVal.(float64), map[string]interface{}{
				"cluster": cluster,
				"job":     job,
				"task":    task,
				"host":    f.host,
			}))
		}
	}
	return result
}
