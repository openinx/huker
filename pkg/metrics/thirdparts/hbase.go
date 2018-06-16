package thirdparts

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"regexp"
	"strings"
	"time"
)

type HBaseMetricFetcher struct {
	url     string
	host    string
	port    int
	job     string
	cluster string
}

func NewHBaseMetricFetcher(url string, host string, port int, cluster string, job string) (*HBaseMetricFetcher, error) {
	return &HBaseMetricFetcher{
		url:     url,
		host:    host,
		port:    port,
		job:     job,
		cluster: cluster,
	}, nil
}

func (f *HBaseMetricFetcher) tags(keyValues map[string]string) map[string]interface{} {
	tagMap := map[string]interface{}{
		"cluster":     f.cluster,
		"hostAndPort": fmt.Sprintf("%s-%d", f.host, f.port),
		"job":         f.job,
	}
	if keyValues != nil {
		for key, val := range keyValues {
			tagMap[key] = val
		}
	}
	return tagMap
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
	if val, ok := bean["ThreadCount"]; ok && val != nil {
		threadCount := val.(float64)
		metricMap := formatMetric("hbase.jvm.ThreadCount", now, threadCount, f.tags(nil))
		result = append(result, metricMap)
	}
	return result
}

const (
	patternParseNsTableMetric = "Namespace_([a-zA-Z0-9_\\-\\.]+)_table_([a-zA-Z0-9_\\-\\.]+)_region_([a-z0-9]+)_metric_([a-zA-Z0-9_]+)"
)

func (f *HBaseMetricFetcher) handleRegionServerRegions(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	namespaceMap := make(map[string]float64)
	tableMap := make(map[string]float64)
	regionServerMap := make(map[string]float64)

	metricKeys := []string{
		"storeCount",
		"storeFileCount",
		"memStoreSize",
		"storeFileSize",
		"readRequestCount",
		"writeRequestCount",
		"get_num_ops",
		"scanNext_num_ops",
		"deleteCount",
		"mutateCount",
	}
	metricMap := make(map[string]bool)
	for _, metricKey := range metricKeys {
		metricMap[metricKey] = true
	}

	for metricName, metricValue := range bean {
		re := regexp.MustCompile(patternParseNsTableMetric)
		matches := re.FindAllStringSubmatch(metricName, -1)
		if len(matches) != 1 || len(matches[0]) != 5 {
			continue
		}
		match := matches[0]
		value := metricValue.(float64)
		namespace, table, encodedRegionName, metric := match[1], match[2], match[3], match[4]
		if _, ok := metricMap[metric]; !ok {
			continue
		}

		// add region point
		tags := f.tags(map[string]string{"regions": encodedRegionName, "namespace": namespace, "table": table})
		result = append(result, formatMetric("hbase.regionserver.regions."+metric, now, value, tags))

		// accumulate the namespace
		nsKey := namespace + "$" + metric
		if val, ok := namespaceMap[nsKey]; !ok {
			namespaceMap[nsKey] = value
		} else {
			namespaceMap[nsKey] = val + value
		}

		// accumulate the table
		tableKey := namespace + "$" + table + "$" + metric
		if val, ok := tableMap[tableKey]; !ok {
			tableMap[tableKey] = value
		} else {
			tableMap[tableKey] = val + value
		}

		// accumulate the region server
		if val, ok := regionServerMap[metric]; !ok {
			regionServerMap[metric] = val
		} else {
			regionServerMap[metric] = val + value
		}
	}

	// add namespace point
	for namespace, value := range namespaceMap {
		parts := strings.Split(namespace, "$")
		if len(parts) == 2 {
			tags := f.tags(map[string]string{"namespace": parts[0]})
			result = append(result, formatMetric("hbase.regionserver.namespace."+parts[1], now, value, tags))
		}
	}

	// add table point
	for table, value := range tableMap {
		parts := strings.Split(table, "$")
		if len(parts) == 3 {
			tags := f.tags(map[string]string{"namespace": parts[0], "table": parts[1]})
			result = append(result, formatMetric("hbase.regionserver.table."+parts[2], now, value, tags))
		}
	}

	// add region server point
	for metric, value := range regionServerMap {
		result = append(result, formatMetric("hbase.regionserver."+metric, now, value, f.tags(nil)))
	}

	return result
}

func (f *HBaseMetricFetcher) handleRegionServerWAL(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	fields := make(map[string]float64)
	for _, key := range []string{
		"SyncTime_num_ops",
		"SyncTime_75th_percentile",
		"SyncTime_90th_percentile",
		"SyncTime_95th_percentile",
		"SyncTime_99th_percentile",
		"AppendTime_num_ops",
		"AppendTime_75th_percentile",
		"AppendTime_90th_percentile",
		"AppendTime_95th_percentile",
		"AppendTime_99th_percentile",
	} {
		if val, ok := bean[key]; ok {
			fields[key] = val.(float64)
		} else {
			log.Infof("key %s not exist in %v", key, f.tags(nil))
		}
	}
	for key, val := range fields {
		metricMap := formatMetric("hbase.regionserver.wal."+key, now, val, f.tags(nil))
		result = append(result, metricMap)
	}
	return result
}

func (f *HBaseMetricFetcher) handleRegionServerServer(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	fields := make(map[string]float64)
	for _, key := range []string{
		"regionCount",
		"storeCount",
		"hlogFileCount",
		"hlogFileSize",
		"storeFileCount",
		"memStoreSize",
		"Mutate_75th_percentile",
		"Mutate_90th_percentile",
		"Mutate_95th_percentile",
		"Mutate_99th_percentile",
		"Increment_75th_percentile",
		"Increment_90th_percentile",
		"Increment_95th_percentile",
		"Increment_99th_percentile",
		"FlushTime_75th_percentile",
		"FlushTime_90th_percentile",
		"FlushTime_95th_percentile",
		"FlushTime_99th_percentile",
		"Delete_75th_percentile",
		"Delete_90th_percentile",
		"Delete_95th_percentile",
		"Delete_99th_percentile",
		"Get_75th_percentile",
		"Get_90th_percentile",
		"Get_95th_percentile",
		"Get_99th_percentile",
		"ScanNext_75th_percentile",
		"ScanNext_90th_percentile",
		"ScanNext_95th_percentile",
		"ScanNext_99th_percentile",
		"Append_75th_percentile",
		"Append_90th_percentile",
		"Append_95th_percentile",
		"Append_99th_percentile",
	} {
		if val, ok := bean[key]; ok {
			fields[key] = val.(float64)
		} else {
			log.Infof("key %s not exist in %v", key, f.tags(nil))
		}
	}

	for key, val := range fields {
		metricMap := formatMetric("hbase.regionserver."+key, now, val, f.tags(nil))
		result = append(result, metricMap)
	}
	return result
}

func (f *HBaseMetricFetcher) handleRegionServerIPC(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	return result
}
