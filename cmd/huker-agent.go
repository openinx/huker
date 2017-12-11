package main

import (
    "gitlab.com/openinx/haloop/haloop"
)

func main() {
    s := haloop.NewSupervisor()
    s.Start()
}
