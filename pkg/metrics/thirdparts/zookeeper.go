package thirdparts

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type ZookeeperMetricFetcher struct {
	host    string
	port    int
	cluster string
}

func NewZookeeperMetricFetcher(cluster, host string, port int) *ZookeeperMetricFetcher {
	return &ZookeeperMetricFetcher{
		cluster: cluster,
		host:    host,
		port:    port,
	}
}

func (f *ZookeeperMetricFetcher) tags() map[string]interface{} {
	return map[string]interface{}{
		"cluster":     f.cluster,
		"hostAndPort": fmt.Sprintf("%s-%d", f.host, f.port),
		"job":         "zkServer",
	}
}

func (f *ZookeeperMetricFetcher) Pull() (interface{}, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", f.host, f.port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("mntr")); err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	var resp string
	if n, err := conn.Read(buf); err != nil {
		return nil, err
	} else {
		resp = string(buf[:n])
	}

	var result []map[string]interface{}
	now := time.Now().Unix()
	for _, part := range strings.Split(resp, "\n") {
		pair := strings.Split(part, "\t")
		if len(pair) != 2 {
			continue
		}
		val, err := strconv.Atoi(pair[1])
		if err != nil {
			continue
		}
		result = append(result, formatMetric("zookeeper.zkServer."+pair[0], now, float64(val), f.tags()))
	}
	return result, nil
}
