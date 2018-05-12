package thirdparts

import (
	"fmt"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
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

func (f *HDFSMetricFetcher) Pull(conf client.BatchPointsConfig) (client.BatchPoints, error) {
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
		} else if bean["name"] == "" {

		}
		if err != nil {
			log.Error("Failed to parse HDFS bean, bean: %s, err: %s", bean, err)
		}
	}
	return bp, nil
}

func (f *HDFSMetricFetcher) handleThreading(bp client.BatchPoints, bean map[string]interface{}) error {
	threadCount := bean["ThreadCount"].(float64)
	pt, err := client.NewPoint("hdfs_jvm", map[string]string{
		"address": f.hostAndPort(),
		"service": "NameNode",
		"key":     "ThreadCount",
		"cluster": f.cluster,
		"type":    "hdfs",
	}, map[string]interface{}{
		"value": threadCount,
	})
	if err != nil {
		return err
	}
	bp.AddPoint(pt)
	return nil
}
