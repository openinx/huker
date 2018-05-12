package thirdparts

import (
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"regexp"
	"strings"
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

func (f *HBaseMetricFetcher) Pull(conf client.BatchPointsConfig) (client.BatchPoints, error) {
	bp, err := client.NewBatchPoints(conf)
	data, err := utils.HttpGetJSON(f.url)
	if err != nil {
		return bp, err
	}

	beans := data["beans"].([]interface{})
	for i := 0; i < len(beans); i++ {
		bean := beans[i].(map[string]interface{})
		var err error
		if bean["name"] == "java.lang:type=Threading" {
			err = f.handleThreading(bp, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=Regions" {
			err = f.handleRegionServerRegions(bp, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=WAL" {
			err = f.handleRegionServerWAL(bp, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=Server" {
			err = f.handleRegionServerServer(bp, bean)
		} else if bean["name"] == "Hadoop:service=HBase,name=RegionServer,sub=IPC" {
			err = f.handleRegionServerIPC(bp, bean)
		}
		if err != nil {
			log.Error("Failed to parse HBase bean, bean: %s, err: %s", bean, err)
		}
	}
	return bp, nil
}

func (f *HBaseMetricFetcher) handleThreading(bp client.BatchPoints, bean map[string]interface{}) error {
	threadCount := bean["ThreadCount"].(float64)
	pt, err := client.NewPoint("hbase_jvm", map[string]string{
		"address": f.hostAndPort(),
		"service": "RegionServer",
		"key":     "ThreadCount",
		"cluster": f.cluster,
		"type":    "hbase",
	}, map[string]interface{}{
		"value": threadCount,
	})
	if err != nil {
		return err
	}
	bp.AddPoint(pt)
	return nil
}

const (
	patternParseNsTableMetric = "Namespace_([a-zA-Z0-9_\\-\\.]+)_table_([a-zA-Z0-9_\\-\\.]+)_region_([a-z0-9]+)_metric_([a-zA-Z0-9_]+)"
)

func (f *HBaseMetricFetcher) handleRegionServerRegions(bp client.BatchPoints, bean map[string]interface{}) error {
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

		//add region point
		pt, err := client.NewPoint("hbase_regions", map[string]string{
			"address": f.hostAndPort(),
			"region":  encodedRegionName,
			"key":     metric,
			"cluster": f.cluster,
			"type":    "hbase",
		}, map[string]interface{}{
			"value": value,
		})
		if err != nil {
			return err
		}
		bp.AddPoint(pt)

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

	//add namespace point
	for namespace, value := range namespaceMap {
		parts := strings.Split(namespace, "$")
		pt, err := client.NewPoint("hbase_namespace", map[string]string{
			"address":   f.hostAndPort(),
			"namespace": parts[0],
			"key":       parts[1],
			"cluster":   f.cluster,
			"type":      "hbase",
		}, map[string]interface{}{
			"value": value,
		})
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}

	// add table point
	for table, value := range tableMap {
		parts := strings.Split(table, "$")
		pt, err := client.NewPoint("hbase_table", map[string]string{
			"address": f.hostAndPort(),
			"table":   parts[0],
			"key":     parts[1],
			"cluster": f.cluster,
			"type":    "hbase",
		}, map[string]interface{}{
			"value": value,
		})
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}

	return nil
}

func (f *HBaseMetricFetcher) handleRegionServerWAL(bp client.BatchPoints, bean map[string]interface{}) error {
	fields := make(map[string]interface{})
	for _, key := range []string{
		"SyncTime_num_ops", "SyncTime_75th_percentile", "SyncTime_90th_percentile", "SyncTime_95th_percentile", "SyncTime_99th_percentile",
		"AppendTime_num_ops", "AppendTime_75th_percentile", "AppendTime_90th_percentile", "AppendTime_95th_percentile", "AppendTime_99th_percentile",
	} {
		fields[key] = bean[key].(float64)
	}
	for key, val := range fields {
		pt, err := client.NewPoint("hbase_wal", map[string]string{
			"address": f.hostAndPort(),
			"service": "RegionServer",
			"key":     key,
			"cluster": f.cluster,
			"type":    "hbase",
		}, map[string]interface{}{
			"value": val,
		})
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}
	return nil
}

func (f *HBaseMetricFetcher) handleRegionServerServer(bp client.BatchPoints, bean map[string]interface{}) error {
	fields := make(map[string]interface{})
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
		pt, err := client.NewPoint("hbase_regionserver", map[string]string{
			"address": f.hostAndPort(),
			"service": "RegionServer",
			"key":     key,
			"cluster": f.cluster,
			"type":    "hbase",
		}, map[string]interface{}{
			"value": val,
		})
		if err != nil {
			return err
		}
		bp.AddPoint(pt)
	}
	return nil
}

func (f *HBaseMetricFetcher) handleRegionServerIPC(bp client.BatchPoints, bean map[string]interface{}) error {
	return nil
}
