package thirdparts

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"strings"
	"time"
)

type HDFSMetricFetcher struct {
	url     string
	host    string
	port    int
	cluster string
	job     string
}

func NewHDFSMetricFetcher(url string, host string, port int, cluster string, job string) (*HDFSMetricFetcher, error) {
	return &HDFSMetricFetcher{
		url:     url,
		host:    host,
		port:    port,
		cluster: cluster,
		job:     job,
	}, nil
}

func (f *HDFSMetricFetcher) hostAndPort() string {
	return fmt.Sprintf("%s:%d", f.host, f.port)
}

func (f *HDFSMetricFetcher) tags() map[string]interface{} {
	return map[string]interface{}{
		"cluster":     f.cluster,
		"job":         f.job,
		"hostAndPort": fmt.Sprintf("%s-%d", f.host, f.port),
	}
}

func (f *HDFSMetricFetcher) Pull() (interface{}, error) {
	data, err := utils.HttpGetJSON(f.url)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	now := time.Now().Unix()

	beans := data["beans"].([]interface{})
	for i := 0; i < len(beans); i++ {
		bean := beans[i].(map[string]interface{})
		nameValue, ok := bean["name"]
		if !ok {
			continue
		}
		name := nameValue.(string)
		if name == "java.lang:type=Threading" {
			result = f.handleThreading(result, now, bean)
		} else if name == "Hadoop:service=NameNode,name=FSNamesystem" { // NameNode
			result = f.handleFSNamesystem(result, now, bean)
		} else if name == "Hadoop:service=NameNode,name=NameNodeActivity" { // NameNode
			if f.isNamenodeWithStandby(bean) {
				// Skip to parse JMX metrics from standby Namenode.
				return nil, nil
			}
			result = f.handleNameNodeActivity(result, now, bean)
		} else if name == "Hadoop:service=NameNode,name=FSNamesystemState" { // NameNode
			result = f.handleFSNamesystemState(result, now, bean)
		} else if strings.HasPrefix(name, "Hadoop:service=DataNode,name=FSDatasetState") { // DataNode
			result = f.handleDataNodeFSDatasetState(result, now, bean)
		} else if strings.HasPrefix(name, "Hadoop:service=DataNode,name=DataNodeActivity") { // DataNode
			result = f.handleDataNodeActivity(result, now, bean)
		}
	}
	return result, nil
}

func (f *HDFSMetricFetcher) isNamenodeWithStandby(bean map[string]interface{}) bool {
	if value, ok := bean["tag.HAState"]; ok && value != "active" {
		return true
	} else {
		return false
	}
}

func (f *HDFSMetricFetcher) handleThreading(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	threadCount := bean["ThreadCount"].(float64)
	result = append(result, formatMetric("hdfs.jvm.ThreadCount", now, threadCount, f.tags()))
	return result
}

func (f *HDFSMetricFetcher) handleFSNamesystem(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	keys := []string{"MissingBlocks", "CapacityTotal", "CapacityUsed", "CapacityRemaining", "BlocksTotal", "FilesTotal"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			result = append(result, formatMetric("hdfs.namenode.fs."+key, now, value.(float64), f.tags()))
		}
	}
	return result
}

func (f *HDFSMetricFetcher) handleNameNodeActivity(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	keys := []string{"SyncsAvgTime", "SyncsNumOps"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			result = append(result, formatMetric("hdfs.namenode.activity."+key, now, value.(float64), f.tags()))
		}
	}
	return result
}

func (f *HDFSMetricFetcher) handleFSNamesystemState(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	keys := []string{"NumLiveDataNodes", "NumDeadDataNodes", "NumDecomLiveDataNodes"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			result = append(result, formatMetric("hdfs.namenode.fs.state."+key, now, value.(float64), f.tags()))
		}
	}
	return result
}

func (f *HDFSMetricFetcher) handleDataNodeFSDatasetState(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	keys := []string{"Capacity", "DfsUsed", "Remaining"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			result = append(result, formatMetric("hdfs.datanode.fs.state."+key, now, value.(float64), f.tags()))
		}
	}
	return result
}

func (f *HDFSMetricFetcher) handleDataNodeActivity(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	keys := []string{"BytesWritten", "BytesRead", "BlocksWritten", "BlocksRead", "ReadsFromLocalClient", "ReadsFromRemoteClient",
		"WritesFromLocalClient", "WritesFromRemoteClient", "FlushNanosNumOps", "FsyncNanosNumOps"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			result = append(result, formatMetric("hdfs.datanode.activity."+key, now, value.(float64), f.tags()))
		}
	}

	keys = []string{"FsyncNanosAvgTime", "FlushNanosAvgTime", "SendDataPacketTransferNanosAvgTime"}
	for _, key := range keys {
		if value, ok := bean[key]; ok {
			keyMs := strings.Replace(key, "Nanos", "", -1)
			result = append(result, formatMetric("hdfs.datanode.activity."+keyMs, now, value.(float64)/1e6, f.tags()))
		}
	}
	return result
}
