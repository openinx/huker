package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	huker "github.com/openinx/huker/pkg/core"
	"github.com/openinx/huker/pkg/metrics/grafana"
	"github.com/openinx/huker/pkg/metrics/thirdparts"
	"github.com/qiniu/log"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type MetricFetcher interface {
	Pull() (interface{}, error)
}

type Collector struct {
	workerSize int

	// OpenTSDB client configurations.
	tsdbHttpAddr string

	// task chan
	tasks chan MetricFetcher

	// Configuration files root directory, use $HUKER_DIR/conf as default.
	cfgRoot string

	// Package Server Http Address for downloading the package libs.
	pkgSrvAddr string

	// To manage all the cluster, job, and host.
	hukerJob *huker.ConfigFileHukerJob

	// Grafana HTTP Server
	grafanaAddr string

	// Grafana  API Key
	grafanaApiKey string

	// Grafana Data Source Name
	grafanaDataSourceKey string

	// Grafana Syncer
	grafanaSyncer *grafana.GrafanaSyncer

	// Period for sync dashboard(seconds), default: 24h
	syncDashboardSeconds time.Duration

	// Period for collect metrics(seconds), default: 5s
	collectSeconds time.Duration
}

func NewCollector(workerSize int, tsdbHttpAddr, cfgRoot, pkgSrvAddr, grafanaAddr, grafanaApiKey string, grafanaDataSourceKey string, syncDashboardSeconds int, collectSeconds int) *Collector {
	hukerJob, err := huker.NewConfigFileHukerJob(cfgRoot, pkgSrvAddr)
	if err != nil {
		panic(err.Error())
	}
	return &Collector{
		workerSize:           workerSize,
		tsdbHttpAddr:         tsdbHttpAddr,
		tasks:                make(chan MetricFetcher),
		cfgRoot:              cfgRoot,
		pkgSrvAddr:           pkgSrvAddr,
		hukerJob:             hukerJob,
		grafanaAddr:          grafanaAddr,
		grafanaApiKey:        grafanaApiKey,
		grafanaDataSourceKey: grafanaDataSourceKey,
		grafanaSyncer:        grafana.NewGrafanaSyncer(grafanaAddr, grafanaApiKey, grafanaDataSourceKey),
		syncDashboardSeconds: time.Duration(syncDashboardSeconds),
		collectSeconds:       time.Duration(collectSeconds),
	}
}

func (c *Collector) fetchAndSave(workId int, m MetricFetcher) {
	// Pull the metric from remote server
	log.Infof("Try to fetch the metrics(workId: %d)...", workId)
	jsonObj, err := m.Pull()
	if err != nil {
		log.Errorf("Failed to pull metric, %s", err)
		return
	}

	// Persist the metric into influxDB.
	if jsonObj == nil {
		log.Warnf("JSON object is nil, skip to persist the points (workId: %d).", workId)
		return
	}

	data, serialError := json.Marshal(jsonObj)
	if serialError != nil {
		log.Errorf("Failed to marshal the interface{} to json string, obj:%v, reason: %v", jsonObj, serialError)
		return
	}

	// New Http Client
	// TODO abstract an OpenTSDB Client.
	req, err0 := http.NewRequest("POST", c.tsdbHttpAddr+"/api/put?details", bytes.NewBuffer(data))
	if err0 != nil {
		log.Errorf("Failed to new http request, url: %s, data: %v", c.tsdbHttpAddr, data)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		log.Errorf("Failed to send json data to OpenTSDB, url: %s, data: %s", req.URL.String(), string(data))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respData, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("Failed to persist json data to OpenTSDB, url: %s, data: %v, reason: %v", req.URL.String(), string(data), string(respData))
		return
	}
}

func (c *Collector) worker(workId int) {
	log.Infof("Worker %d started", workId)
	for task := range c.tasks {
		c.fetchAndSave(workId, task)
	}
}

func getHostJMXAddr(host *huker.Host) string {
	return fmt.Sprintf("http://%s:%d/jmx", host.Hostname, host.BasePort+1)
}

func parseHostName(host string) string {
	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[0:idx]
	}
	return strings.Split(host, ":")[0]
}

func (c *Collector) createHostDashboard() {
	hosts, err := c.hukerJob.ListHosts()
	if err != nil {
		log.Errorf("Failed to list all supervisor agents, %v", err)
		return
	}

	// Dashboard for each host
	for _, host := range hosts {
		err := c.grafanaSyncer.CreateHostDashboard(parseHostName(host))
		if err != nil {
			log.Errorf("Failed to create host dashboard for host: %s, %v", host, err)
			continue
		}
	}
}

func (c *Collector) createNodesDashboard() {
	clusters, err := c.hukerJob.List()
	if err != nil {
		log.Errorf("Failed to list all clusters, %v", err)
		return
	}
	hostNames := make(map[string]bool)
	for _, cluster := range clusters {
		for _, job := range cluster.Jobs {
			for _, host := range job.Hosts {
				hostNames[host.ToHttpAddress()] = true
			}
		}
		var hostStrings []string
		for host := range hostNames {
			hostStrings = append(hostStrings, parseHostName(host))
		}
		err := c.grafanaSyncer.CreateNodesDashboard(cluster.ClusterName, hostStrings)
		if err != nil {
			log.Errorf("Failed to create nodes dashboard for cluster(%s), host: %v, %v", cluster.ClusterName, hostStrings, err)
			continue
		}
	}
}

func (c *Collector) createClusterDashboard() {
	clusters, err := c.hukerJob.List()
	if err != nil {
		log.Errorf("Failed to list all clusters, %v", err)
		return
	}
	for _, cluster := range clusters {
		if cluster.Project == "hdfs" {
			err := c.grafanaSyncer.CreateHDFSDashboard(cluster)
			if err != nil {
				log.Errorf("Failed to create hdfs dashboard, %v", err)
			}
		} else if cluster.Project == "zookeeper" {
			err := c.grafanaSyncer.CreateZookeeperDashboard(cluster)
			if err != nil {
				log.Errorf("Failed to create zookeeper dashboard, %v", err)
			}
		}
	}
}

func (c *Collector) syncGrafanaDashboard() {
	for {
		// Dashboard for each host
		c.createHostDashboard()

		// Integrate Nodes Dashboard for each cluster
		c.createNodesDashboard()

		// Dashboard for each cluster
		c.createClusterDashboard()

		time.Sleep(c.syncDashboardSeconds * time.Second)
	}
}

func (c *Collector) Start() {
	// Start a goroutine to sync the grafana dashboard
	go c.syncGrafanaDashboard()

	for i := 0; i < c.workerSize; i++ {
		go c.worker(i)
	}

	for {
		time.Sleep(c.collectSeconds * time.Second)

		clusters, err := c.hukerJob.List()
		if err != nil {
			log.Errorf("Failed to list all config files, %v", err)
			continue
		}

		if hosts, err := c.hukerJob.ListHosts(); err != nil {
			log.Errorf("Failed to list all supervisor agents, %v", err)
			continue
		} else {
			for _, host := range hosts {
				f, err := thirdparts.NewNodeMetricFetcher(host+"/api/metrics", parseHostName(host))
				if err != nil {
					log.Errorf("Failed to initialize the NodeMetricFetcher for %s, %v", host, err)
					continue
				}
				c.tasks <- f
			}
		}

		for _, cluster := range clusters {
			for jobName, job := range cluster.Jobs {
				if job.Hosts != nil && len(job.Hosts) > 0 {
					if jobName == "regionserver" {
						// HBase RegionServer Metrics
						for _, host := range job.Hosts {
							f, err := thirdparts.NewHBaseMetricFetcher(getHostJMXAddr(host), host.Hostname, host.BasePort+1, cluster.ClusterName)
							if err != nil {
								log.Errorf("Failed to initialize the HBaseMetricFetcher for %s, %v", host.ToKey(), err)
								continue
							}
							c.tasks <- f
						}
					} else if jobName == "namenode" || jobName == "datanode" {
						// HDFS NameNode Metrics
						for _, host := range job.Hosts {
							f, err := thirdparts.NewHDFSMetricFetcher(getHostJMXAddr(host), host.Hostname, host.BasePort+1, cluster.ClusterName, jobName)
							if err != nil {
								log.Errorf("Failed to initialize the NewHDFSMetricFetcher for %s, %v", host.ToKey(), err)
								continue
							}
							c.tasks <- f
						}
					} else if jobName == "zkServer" {
						for _, host := range job.Hosts {
							c.tasks <- thirdparts.NewZookeeperMetricFetcher(cluster.ClusterName, host.Hostname, host.BasePort)
						}
					}
				}
			}
		}
	}
}
