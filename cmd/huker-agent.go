package main

import (
	"fmt"
	"github.com/openinx/huker"
	"github.com/qiniu/log"
	"time"
)

func main() {
	s, err := huker.NewSupervisor(fmt.Sprintf("/Users/openinx/test/%d", int32(time.Now().Unix())),
		9001,
		"/Users/openinx/test/supervisor.db")

	if err != nil {
		log.Fatal(err)
	}
	s.Start()
}
