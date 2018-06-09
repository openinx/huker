package thirdparts

import (
	"fmt"
	"github.com/openinx/huker/pkg/utils"
	"time"
)

type HDFSMetricFetcher struct {
	url     string
	host    string
	port    int
	cluster string
}

func NewHDFSMetricFetcher(url string, host string, port int, cluster string) (*HDFSMetricFetcher, error) {
	return &HDFSMetricFetcher{
		url:     url,
		host:    host,
		port:    port,
		cluster: cluster,
	}, nil
}

func (f *HDFSMetricFetcher) hostAndPort() string {
	return fmt.Sprintf("%s:%d", f.host, f.port)
}

func (f *HDFSMetricFetcher) tags() map[string]interface{} {
	return map[string]interface{}{
		"cluster": f.cluster,
		"host":    f.host,
		"port":    f.port,
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
		if bean["name"] == "java.lang:type=Threading" {
			result = f.handleThreading(result, now, bean)
		}
		// TODO parse more beans ...
	}
	return result, nil
}

func (f *HDFSMetricFetcher) handleThreading(result []map[string]interface{}, now int64, bean map[string]interface{}) []map[string]interface{} {
	threadCount := bean["ThreadCount"].(float64)
	result = append(result, formatMetric("hdfs.namenode.jvm", now, threadCount, f.tags()))
	return result
}
