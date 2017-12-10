package main

import (
    "fmt"
    "path"
    "io/ioutil"
    "github.com/qiniu/log"
    "github.com/go-yaml/yaml"
)

const CONF_ROOT = "."

func main() {
    data, err := ioutil.ReadFile(path.Join(CONF_ROOT, "conf/hbase/common/common.yaml"))
    if err != nil {
        log.Errorf("Read pkg.yaml failed: %v", err)
        return
    }

    config := make(map[interface{}]interface{})
    if err := yaml.Unmarshal(data, &config); err != nil {
        log.Errorf("Deserialize yaml config error: %v", err)
        return
    }
    fmt.Printf("Hello world, %v\n", config)

}