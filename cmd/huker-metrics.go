package main

import (
	"github.com/openinx/huker/pkg/metrics"
)

func main() {
	collector := metrics.NewCollector(10, "http://localhost:51001/api/put?details")
	collector.Start()
}
