package main

import (
	"github.com/openinx/huker/pkg"
	"github.com/openinx/huker/pkg/minihuker"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"net/url"
	"path"
	"strconv"
)

func main() {
	var pkgSrvURL, dashboardURL *url.URL
	var err error

	hukerDir := utils.GetHukerDir()
	hukerYamlFile := path.Join(hukerDir, "conf", "huker.yaml")
	cfg, err := pkg.NewHukerConfig(hukerYamlFile)
	if err != nil {
		log.Fatalf("Failed to parse huker config file %s, %v", hukerYamlFile, err)
		return
	}

	pkgSrvURL, err = cfg.GetURL(pkg.HukerPkgSrvHttpAddress)
	if err != nil {
		log.Fatal(err)
		return
	}
	pkgSrvPort, _ := strconv.Atoi(pkgSrvURL.Port())

	dashboardURL, err = cfg.GetURL(pkg.HukerDashboardHttpAddress)
	if err != nil {
		log.Error(err)
		return
	}
	dashboardPort, _ := strconv.Atoi(dashboardURL.Port())

	huker := minihuker.NewMiniHuker(path.Join(hukerDir, "conf"),
		1,
		path.Join(hukerDir, "data"),
		cfg.GetInt(pkg.HukerSupervisorPort),
		pkgSrvPort,
		path.Join(hukerDir, "lib"),
		path.Join(hukerDir, "conf/pkg.yaml"),
		dashboardPort,
		cfg.Get(pkg.HukerGrafanaHttpAddress))

	huker.Start()
	huker.Wait()
}
