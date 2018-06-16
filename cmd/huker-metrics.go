package main

import (
	"github.com/openinx/huker/pkg"
	"github.com/openinx/huker/pkg/metrics"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"path"
)

func startCollector() {
	hukerDir := utils.GetHukerDir()
	hukerYamlFile := path.Join(hukerDir, "conf", "huker.yaml")
	cfg, err := pkg.NewHukerConfig(hukerYamlFile)
	if err != nil {
		log.Fatalf("Failed to parse huker config file %s, %v", hukerYamlFile, err)
		return
	}

	workSize := cfg.GetInt(pkg.HukerCollectorWorkerSize)
	openTSDBHttpAddr := cfg.Get(pkg.HukerOpenTSDBHttpAddress)
	pkgsrvAddr := cfg.Get(pkg.HukerPkgSrvHttpAddress)
	grafanaHttpAddr := cfg.Get(pkg.HukerGrafanaHttpAddress)
	grafanaApiKey := cfg.Get(pkg.HukerGrafanaAPIKey)
	grafanaDataSource := cfg.Get(pkg.HukerGrafanaDataSource)
	syncDashboardSeconds := cfg.GetInt(pkg.HukerCollectorSyncDashboardSeconds)
	collectSeconds := cfg.GetInt(pkg.HukerCollectorCollectSeconds)

	collector := metrics.NewCollector(workSize,
		openTSDBHttpAddr,
		path.Join(hukerDir, "conf"),
		pkgsrvAddr,
		grafanaHttpAddr,
		grafanaApiKey,
		grafanaDataSource,
		syncDashboardSeconds,
		collectSeconds,
		cfg)

	collector.Start()
}

func main() {
	startCollector()
}
