package main

import (
	"github.com/openinx/huker"
	"github.com/qiniu/log"
)

func main() {
	log.Infof("Start package manager server, listen port 4000 ...")
	huker.StartPkgManager(":4000", "./testdata/conf/pkg.yaml", "./testdata/lib")
}
