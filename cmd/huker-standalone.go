package main

import (
	"github.com/openinx/huker/pkg/minihuker"
	"github.com/openinx/huker/pkg/utils"
)

func main() {
	hukerDir := utils.GetHukerDir()
	huker := minihuker.NewRawMiniHuker(1, hukerDir+"/data", 9001,
		4000, hukerDir+"/lib", hukerDir+"/conf/pkg.yaml", 8001)
	huker.Start()
	huker.Wait()
}
