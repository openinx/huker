package metrics

import (
	"bytes"
	"encoding/json"
	"github.com/openinx/huker/pkg/metrics/thirdparts"
	"github.com/qiniu/log"
	"io/ioutil"
	"net/http"
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
}

func NewCollector(workerSize int, tsdbHttpAddr string) *Collector {
	return &Collector{
		workerSize:   workerSize,
		tsdbHttpAddr: tsdbHttpAddr,
		tasks:        make(chan MetricFetcher),
	}
}

func (c *Collector) fetchAndSave(workId int, m MetricFetcher) {
	// Pull the metric from remote server
	log.Infof("Try to fetch the metrics(workId: %d)...", workId)
	jsonObj, err := m.Pull()
	if err != nil {
		log.Errorf("Failed to pull metric, %s", err)
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
	req, err0 := http.NewRequest("POST", c.tsdbHttpAddr, bytes.NewBuffer(data))
	if err0 != nil {
		log.Errorf("Failed to new http request, url: %s, data: %v", c.tsdbHttpAddr, data)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		log.Errorf("Send json data to OpenTSDB failed, url: %s, data: %s", c.tsdbHttpAddr, string(data))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respData, _ := ioutil.ReadAll(resp.Body)
		log.Errorf("Failed to persist json data to OpenTSDB, url: %s, data: %v, reason: %v", c.tsdbHttpAddr, string(data), string(respData))
		return
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

		f1, err1 := thirdparts.NewHBaseMetricFetcher("http://127.0.0.1:31001/jmx", "127.0.0.1", 64916, "test-hbase")
		if err1 != nil {
			log.Errorf("Failed to initialize hbase fetcher, error: %v", err1)
			continue
		}
		c.tasks <- f1

		f2, err2 := thirdparts.NewHDFSMetricFetcher("http://127.0.0.1:20101/jmx", "127.0.0.1", 20101, "test-hdfs")
		if err2 != nil {
			log.Errorf("Failed to initialize hdfs fetcher, error: %v", err2)
			continue
		}
		c.tasks <- f2

		time.Sleep(1 * time.Second)
	}
}
