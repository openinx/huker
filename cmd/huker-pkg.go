package main

import (
    "gitlab.com/openinx/haloop/haloop"
    "github.com/qiniu/log"
)

func main() {
    log.Infof("Start package manager server, listen port 4000 ...")
    haloop.StartPkgManager(":4000", "./conf/pkg.yaml", "./lib")
}