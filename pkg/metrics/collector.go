package metrics

import (
	"github.com/influxdata/influxdb/client/v2"
	"github.com/openinx/huker/pkg/metrics/thirdparts"
	"github.com/qiniu/log"
	"time"
)

type InfluxDBClient struct {
	Client   client.Client
	Database string
}

func NewInfluxDBClient(addr string, username string, password string, database string) (*InfluxDBClient, error) {
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     addr,
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, err
	}
	return &InfluxDBClient{
		Client:   c,
		Database: database,
	}, nil
}

func (c *InfluxDBClient) NewBatchPointsConfig() client.BatchPointsConfig {
	return client.BatchPointsConfig{
		Database:  c.Database,
		Precision: "s",
	}
}

func (c *InfluxDBClient) Close() error {
	return c.Client.Close()
}

type MetricFetcher interface {
	Pull(conf client.BatchPointsConfig) (client.BatchPoints, error)
}

type Collector struct {
	workerSize int

	// InfluxDB client configurations.
	influxServerAddr string
	influxUserName   string
	influxPassword   string
	influxDatabase   string

	// task chan
	tasks chan MetricFetcher
}

func NewCollector(workerSize int, influxServerAddr string, influxUserName string, influxPassword string, influxDatabase string) *Collector {
	return &Collector{
		workerSize:       workerSize,
		influxServerAddr: influxServerAddr,
		influxUserName:   influxUserName,
		influxPassword:   influxPassword,
		influxDatabase:   influxDatabase,
		tasks:            make(chan MetricFetcher),
	}
}

func (c *Collector) fetchAndSave(workId int, m MetricFetcher) {
	// Initialize the influxDB client.
	influxCli, err := NewInfluxDBClient(c.influxServerAddr, c.influxUserName, c.influxPassword, c.influxDatabase)
	if err != nil {
		log.Stack(err)
		return
	}
	defer influxCli.Close()

	// Pull the metric from remote server
	log.Infof("Try to fetch the metrics(workId: %d)...", workId)
	bp, err := m.Pull(influxCli.NewBatchPointsConfig())
	if err != nil {
		log.Errorf("Failed to pull metric, %s", err)
	}

	// Persist the metric into influxDB.
	if bp == nil {
		log.Warnf("Batch points is nil, skip to persist the points (workId: %d).", workId)
		return
	}

	if err := influxCli.Client.Write(bp); err != nil {
		log.Errorf("Failed to persist the batch points to influxdb database, err: %v", err)
	}
}

func (c *Collector) worker(workId int) {
	log.Infof("Worker %d started", workId)
	for task := range c.tasks {
		c.fetchAndSave(workId, task)
	}
}

func (c *Collector) Start() {
	for i := 0; i < c.workerSize; i++ {
		go c.worker(i)
	}

	for {
		url := "http://127.0.0.1:9001/api/metrics"
		f, err := thirdparts.NewNodeMetricFetcher(url, "127.0.0.1")
		if err != nil {
			log.Errorf("Failed to initialize fetcher, error: %v", err)
			continue
		}
		c.tasks <- f

		f1, err1 := thirdparts.NewHBaseMetricFetcher("http://127.0.0.1:64918/jmx", "127.0.0.1", 64916, "test-hbase")
		if err1 != nil {
			log.Errorf("Failed to initialize hbase fetcher, error: %v", err1)
			continue
		}
		c.tasks <- f1

		time.Sleep(1 * time.Second)
	}
}
