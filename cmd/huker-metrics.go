package main

import (
	"github.com/openinx/huker/pkg/core"
	"github.com/openinx/huker/pkg/metrics"
	"github.com/openinx/huker/pkg/utils"
	"path"
)

func main() {
	hukerDir := utils.GetHukerDir()
	collector := metrics.NewCollector(10,
		"http://127.0.0.1:51001/api/put?details",
		path.Join(hukerDir, core.HUKER_CONF_DIR_DEFAULT),
		"http://127.0.0.1:4000",
		"http://127.0.0.1:3000",
		"Bearer eyJrIjoiSW9JelRJd2xSN3c2ZGZEMVBuUXdhbFJJQ0txR2pqR2wiLCJuIjoiaHVrZXIiLCJpZCI6MX0=",
		"test-opentsdb")
	collector.Start()
}
