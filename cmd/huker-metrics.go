package main

import (
	"github.com/openinx/huker/pkg/metrics"
)

func main() {
	collector := metrics.NewCollector(10, "http://localhost:8086", "admin", "admin", "granfa")
	collector.Start()
}
