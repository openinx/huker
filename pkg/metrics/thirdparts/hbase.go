package thirdparts

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"regexp"
	"strings"
	"time"
)

type HBaseMetricFetcher struct {
	url     string
	host    string
	port    int
	cluster string
}

func NewHBaseMetricFetcher(url string, host string, port int, cluster string) (*HBaseMetricFetcher, error) {
	return &HBaseMetricFetcher{
		url:     url,
		host:    host,
		port:    port,
		cluster: cluster,
	}, nil
}

func (f *HBaseMetricFetcher) hostAndPort() string {
	return fmt.Sprintf("%s:%d", f.host, f.port)
}

func (f *HBaseMetricFetcher) tags() map[string]interface{} {
	return map[string]interface{}{
		"cluster": f.cluster,
		"host":    f.host,
		"port":    f.port,
	}
}

func (f *HBaseMetricFetcher) Pull() (interface{}, error) {
	data, err := utils.HttpGetJSON(f.url)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	now := time.Now().Unix()

	beans := data["beans"].([]interface{})
	for i := 0; i < len(beans); i++ {
		bean := beans[i].(map[string]interface{})
		if bean["name"] == "java.lang:type=Threading" {
			result = f.handleThreading(result, now, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=Regions" {
			result = f.handleRegionServerRegions(result, now, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=WAL" {
			result = f.handleRegionServerWAL(result, now, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=Server" {
			result = f.handleRegionServerServer(result, now, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=IPC" {
			result = f.handleRegionServerIPC(result, now, bean)
		}
	}
	return result, nil
}

func (f *HBaseMetricFetcher) handleThreading(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	threadCount := bean["ThreadCount"].(float64)
	metricMap := formatMetric("hbase.regionserver.jvm", now, threadCount, f.tags())
	return append(result, metricMap)
}

const (
	patternParseNsTableMetric = "Namespace_([a-zA-Z0-9_\\-\\.]+)_table_([a-zA-Z0-9_\\-\\.]+)_region_([a-z0-9]+)_metric_([a-zA-Z0-9_]+)"
)

func (f *HBaseMetricFetcher) handleRegionServerRegions(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	namespaceMap := make(map[string]float64)
	tableMap := make(map[string]float64)

	metricMap := map[string]bool{"storeCount": true, "storeFileCount": true, "memStoreSize": true,
		"storeFileSize": true, "readRequestCount": true, "writeRequestCount": true,
		"get_num_ops": true, "scanNext_num_ops": true, "deleteCount": true, "mutateCount": true}
	for metricName, metricValue := range bean {
		re := regexp.MustCompile(patternParseNsTableMetric)
		matches := re.FindAllStringSubmatch(metricName, -1)
		if len(matches) != 1 {
			continue
		}
		match := matches[0]
		value := metricValue.(float64)
		namespace, table, encodedRegionName, metric := match[1], match[2], match[3], match[4]
		if _, ok := metricMap[metric]; !ok {
			continue
		}

		// add region point
		result = append(result, formatMetric("hbase.regionserver.regions."+encodedRegionName, now, value, f.tags()))

		// accumulate the namespace
		nsKey := namespace + "$" + metric
		if val, ok := namespaceMap[nsKey]; !ok {
			namespaceMap[nsKey] = value
		} else {
			namespaceMap[nsKey] = val + value
		}

		// accumulate the table
		tableKey := namespace + ":" + table + "$" + metric
		if val, ok := tableMap[tableKey]; !ok {
			tableMap[tableKey] = value
		} else {
			tableMap[tableKey] = val + value
		}
	}

	// add namespace point
	for namespace, value := range namespaceMap {
		parts := strings.Split(namespace, "$")
		result = append(result, formatMetric("hbase.regionserver.namespace."+parts[0]+"."+parts[1], now, value, f.tags()))
	}

	// add table point
	for table, value := range tableMap {
		parts := strings.Split(table, "$")
		result = append(result, formatMetric("hbase.regionserver.table."+parts[0]+"."+parts[1], now, value, f.tags()))
	}

	return result
}

func (f *HBaseMetricFetcher) handleRegionServerWAL(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	fields := make(map[string]float64)
	for _, key := range []string{
		"SyncTime_num_ops", "SyncTime_75th_percentile", "SyncTime_90th_percentile", "SyncTime_95th_percentile", "SyncTime_99th_percentile",
		"AppendTime_num_ops", "AppendTime_75th_percentile", "AppendTime_90th_percentile", "AppendTime_95th_percentile", "AppendTime_99th_percentile",
	} {
		fields[key] = bean[key].(float64)
	}
	for key, val := range fields {
		metricMap := formatMetric("hbase.regionserver.wal."+key, now, val, f.tags())
		result = append(result, metricMap)
	}
	return result
}

func (f *HBaseMetricFetcher) handleRegionServerServer(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	fields := make(map[string]float64)
	for _, key := range []string{
		"regionCount", "storeCount", "hlogFileCount", "hlogFileSize", "storeFileCount", "memStoreSize",
		"Mutate_75th_percentile", "Mutate_90th_percentile", "Mutate_95th_percentile", "Mutate_99th_percentile",
		"Increment_75th_percentile", "Increment_90th_percentile", "Increment_95th_percentile", "Increment_99th_percentile",
		"FlushTime_75th_percentile", "FlushTime_90th_percentile", "FlushTime_95th_percentile", "FlushTime_99th_percentile",
		"Delete_75th_percentile", "Delete_90th_percentile", "Delete_95th_percentile", "Delete_99th_percentile",
		"Get_75th_percentile", "Get_90th_percentile", "Get_95th_percentile", "Get_99th_percentile",
		"ScanNext_75th_percentile", "ScanNext_90th_percentile", "ScanNext_95th_percentile", "ScanNext_99th_percentile",
		"Append_75th_percentile", "Append_90th_percentile", "Append_95th_percentile", "Append_99th_percentile",
	} {
		fields[key] = bean[key].(float64)
	}

	for key, val := range fields {
		metricMap := formatMetric("hbase.regionserver."+key, now, val, f.tags())
		result = append(result, metricMap)
	}
	return result
}

func (f *HBaseMetricFetcher) handleRegionServerIPC(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	return result
}
