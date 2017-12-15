package main

import (
	"github.com/openinx/huker"
	"github.com/qiniu/log"
)

func main() {
	log.Infof("Start package manager server, listen port 4000 ...")

	p, err := huker.NewPackageServer("0.0.0.0:4000", "./testdata/lib", "./testdata/conf/pkg.yaml")
	if err != nil {
		log.Error(err)
	}

	if err1 := p.Start(); err1 != nil {
		log.Error(err1)
	}
}
