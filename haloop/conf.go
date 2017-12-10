package haloop

import (
    "reflect"
    "io/ioutil"
    "github.com/go-yaml/yaml"
    "fmt"
    "strings"
    "github.com/qiniu/log"
)

const SHELL_SEPARATOR = " "

/********************************** ConfigFile Implementation *****************************/
type ConfigFile interface {
    toString() string
    toKeyValue() map[string]string
    getConfigName() string
}

type INIConfigFile struct {
    cfgName   string
    keyValues []string
}

func NewINIConfigFile(cfgName string, keyValues []string) INIConfigFile {
    return INIConfigFile{
        cfgName:cfgName,
        keyValues:keyValues,
    }
}

func (c INIConfigFile) toString() string {
    return strings.Join(c.keyValues, "\n")
}

func (c INIConfigFile) toKeyValue() map[string]string {
    ret := make(map[string]string)
    for i := range c.keyValues {
        parts := strings.Split(c.keyValues[i], "=")
        if len(parts) != 2 {
            panic(fmt.Sprintf("Invalid key value pair, key or value not found. %s", c.keyValues[i]))
        }
        ret[parts[0]] = parts[1]
    }
    return ret
}

func (c INIConfigFile) getConfigName() string {
    return c.cfgName
}

type XMLConfigFile struct {
    cfgName   string
    keyValues []string
}

func NewXMLConfigFile(cfgName string, keyValues []string) XMLConfigFile {
    return XMLConfigFile{
        cfgName:cfgName,
        keyValues:keyValues,
    }
}

func (c XMLConfigFile) toString() string {
    var buf []string
    buf = append(buf, "<configuration>")

    kvMap := c.toKeyValue()
    for key := range kvMap {
        buf = append(buf, "  <property>")
        buf = append(buf, fmt.Sprintf("    <name>%s</name>", key))
        buf = append(buf, fmt.Sprintf("    <value>%s</value>", kvMap[key]))
        buf = append(buf, "  </property>")
    }
    buf = append(buf, "</configuration>")

    return strings.Join(buf, "\n")
}

func (c XMLConfigFile) toKeyValue() map[string]string {
    ret := make(map[string]string)
    for i := range c.keyValues {
        parts := strings.Split(c.keyValues[i], "=")
        if len(parts) != 2 {
            panic(fmt.Sprintf("Invalid key value pair, key or value not found. %s", c.keyValues[i]))
        }
        ret[parts[0]] = parts[1]
    }
    return ret
}

func (c XMLConfigFile) getConfigName() string {
    return c.cfgName
}

/********************************** ServiceConfig Implementation *****************************/

type MainEntry struct {
    javaClass string
    extraArgs string
}

func (m MainEntry) toShell() string {
    return strings.Join([]string{m.javaClass, m.extraArgs}, SHELL_SEPARATOR)
}

type Job struct {
    jobName       string
    basePort      int
    hosts         []string
    jvmOpts       []string
    jvmProperties []string
    classpath     []string
    configFiles   map[string]ConfigFile
    mainEntry     MainEntry
}

func (job Job) toShell() string {
    var buf []string
    for i := range job.jvmOpts {
        jvmOpt := strings.TrimSpace(job.jvmOpts[i])
        if len(jvmOpt) > 0 {
            buf = append(buf, jvmOpt)
        }
    }
    for i := range job.jvmProperties {
        jvmPro := strings.TrimSpace(job.jvmProperties[i])
        if len(jvmPro) > 0 {
            buf = append(buf, fmt.Sprintf("-D%s", jvmPro))
        }
    }

    var classpath []string
    for i := range job.classpath {
        cp := strings.TrimSpace(job.classpath[i])
        if len(cp) > 0 {
            classpath = append(classpath, cp)
        }
    }
    buf = append(buf, "-cp")
    buf = append(buf, strings.Join(classpath, ":"))

    buf = append(buf, job.mainEntry.toShell())
    return strings.Join(buf, SHELL_SEPARATOR)
}

type ServiceConfig struct {
    baseConfig    string
    clusterName   string
    javaHome      string
    packageName   string
    packageMd5sum string
    jobs          map[string]Job
}

func readYamlConfig(yamlCfgPath string) ([]string, error) {
    data, err := ioutil.ReadFile(yamlCfgPath)
    if err != nil {
        return []string{}, err
    }
    cfgMap := make(map[interface{}]interface{})
    if err := yaml.Unmarshal(data, &cfgMap); err != nil {
        return []string{}, err
    }
    strings := []string{string(data)}
    baseCfg := cfgMap["base"]
    if baseCfg == nil {
        return strings, nil
    }else {
        if reflect.TypeOf(baseCfg).Kind() != reflect.String {
            return []string{}, fmt.Errorf("base configuration path is not a string: %v", baseCfg)
        }
        baseConfigs, err2 := readYamlConfig(baseCfg.(string))
        if err2 != nil {
            return []string{}, err2
        }
        for i := range baseConfigs {
            strings = append(strings, baseConfigs[i])
        }
        return strings, nil
    }
}

func parseStringArray(s interface{}) ([]string, error) {
    typeOf := reflect.TypeOf(s)
    if s != nil && (typeOf.Kind() == reflect.Array || typeOf.Kind() == reflect.Slice) {
        array := s.([]interface{})
        var strings []string
        for i := range array {
            strings = append(strings, array[i].(string))
        }
        return strings, nil
    }
    return []string{}, fmt.Errorf("Not a Array type, type: %v, content: %v", typeOf, s)
}

func parseMainEntry(s interface{}) (MainEntry, error) {
    if s == nil || reflect.TypeOf(s).Kind() != reflect.Map {
        return MainEntry{}, fmt.Errorf("Invalid main entry, not a map")
    }
    mainEntry := MainEntry{}
    meMap := s.(map[interface{}]interface{})
    if meMap["java_class"] != nil {
        mainEntry.javaClass = meMap["java_class"].(string)
    }
    if meMap["extra_args"] != nil {
        mainEntry.extraArgs = meMap["extra_args"].(string)
    }
    return mainEntry, nil
}

func parseConfigFile(cfgName string, keyValues []string) (ConfigFile, error) {
    if strings.HasSuffix(cfgName, ".cfg") {
        return NewINIConfigFile(cfgName, keyValues), nil
    }else if (strings.HasSuffix(cfgName, ".xml")) {
        return NewXMLConfigFile(cfgName, keyValues), nil
    }else {
        return nil, fmt.Errorf("Unsupported configuration file format. %s", cfgName)
    }
    return nil, nil
}

func parseConfigFileArray(s interface{}) ([]ConfigFile, error) {
    if s == nil || reflect.TypeOf(s).Kind() != reflect.Map {
        return nil, fmt.Errorf("Invalid config files array , not a map")
    }
    cfMap := s.(map[interface{}]interface{})
    var cfgFiles []ConfigFile
    for key := range cfMap {
        cfgName := key.(string)
        if cfMap[key] == nil {
            log.Warnf("Configuration file has no key-value pairs. %s", key)
            continue
        }
        if keyValues, err := parseStringArray(cfMap[key]); err != nil {
            return nil, err
        }else {
            if cf, err := parseConfigFile(cfgName, keyValues); err != nil {
                return nil, err
            }else {
                cfgFiles = append(cfgFiles, cf)
            }
        }
    }
    return cfgFiles, nil
}

func NewJob(jobName string, jobMap map[interface{}]interface{}) (Job, error) {
    job := Job{
        jobName: jobName,
    }
    if jobMap["base_port"] != nil {
        job.basePort = jobMap["base_port"].(int)
    }
    var err error
    if jobMap["hosts"] != nil {
        if job.hosts, err = parseStringArray(jobMap["hosts"]); err != nil {
            return Job{}, err
        }
    }
    if jobMap["jvm_opts"] != nil {
        if job.jvmOpts, err = parseStringArray(jobMap["jvm_opts"]); err != nil {
            return Job{}, err
        }
    }
    if jobMap["jvm_properties"] != nil {
        if job.jvmProperties, err = parseStringArray(jobMap["jvm_properties"]); err != nil {
            return Job{}, err
        }
    }
    if jobMap["classpath"] != nil {
        if job.classpath, err = parseStringArray(jobMap["classpath"]); err != nil {
            return Job{}, err
        }
    }
    if jobMap["config"] != nil {
        if confFiles, err2 := parseConfigFileArray(jobMap["config"]); err2 != nil {
            return Job{}, err2
        }else {
            job.configFiles = make(map[string]ConfigFile)
            for i := range confFiles {
                job.configFiles[confFiles[i].getConfigName()] = confFiles[i]
            }
        }
    }

    if jobMap["main_entry"] != nil {
        if job.mainEntry, err = parseMainEntry(jobMap["main_entry"]); err != nil {
            return Job{}, err
        }
    }
    return job, nil
}

// There are many .yaml configurations, for example yamlConfigs[0], yamlConfigs[1], ..., which the first conf will merge
// with the second one (the first win if encounter a same key), the second will merge with the third one.
func mergeMultipleYamlConfig(yamlConfigs []string) (map[interface{}]interface{}, error) {
    if len(yamlConfigs) == 0 {
        return map[interface{}]interface{}{}, fmt.Errorf("No yaml config provided.")
    }
    yamlMaps := make([]map[interface{}]interface{}, len(yamlConfigs))
    for i := range yamlConfigs {
        yamlMaps[i] = map[interface{}]interface{}{}
        if err := yaml.Unmarshal([]byte(yamlConfigs[i]), &yamlMaps[i]); err != nil {
            return map[interface{}]interface{}{}, fmt.Errorf("Unmarshal yaml error, config: %s, cause: %v", yamlConfigs[i], err)
        }
    }
    ret := yamlMaps[0]
    for i := 1; i < len(yamlMaps); i++ {
        ret = MergeYamlMap(ret, yamlMaps[i])
    }
    return ret, nil
}

func LoadServiceConfig(yamlCfgPath string) (*ServiceConfig, error) {
    if cfgs, err := readYamlConfig(yamlCfgPath); err != nil {
        return nil, err
    }else {
        return NewServiceConfig(cfgs)
    }
}

func NewServiceConfig(yamlConfigs []string) (*ServiceConfig, error) {
    cfgMap, err := mergeMultipleYamlConfig(yamlConfigs)
    if err != nil {
        return nil, err
    }
    svCfg := &ServiceConfig{}
    if cfgMap["base"] != nil {
        svCfg.baseConfig = cfgMap["base"].(string)
    }

    if cfgMap["cluster"] == nil {
        return nil, fmt.Errorf("`cluster` key does not exists.")
    }
    if reflect.TypeOf(cfgMap["cluster"]).Kind() != reflect.Map {
        return nil, fmt.Errorf("`cluster` key format is incorrect.")
    }

    getRequiredField := func(m map[interface{}]interface{}, key string) (string, error) {
        if _, ok := m[key]; !ok {
            return "", fmt.Errorf("Required key `%s` in configuration file does not exist.", key)
        }
        if reflect.TypeOf(m[key]).Kind() != reflect.String {
            return "", fmt.Errorf("`%s` should be a string, not %s", key, reflect.TypeOf(m[key]))
        }
        return m[key].(string), nil
    }

    cluster := cfgMap["cluster"].(map[interface{}]interface{})
    if svCfg.clusterName, err = getRequiredField(cluster, "cluster_name"); err != nil {
        return nil, err
    }
    if svCfg.javaHome, err = getRequiredField(cluster, "java_home"); err != nil {
        return nil, err
    }
    if svCfg.packageName, err = getRequiredField(cluster, "package_name"); err != nil {
        return nil, err
    }
    if svCfg.packageMd5sum, err = getRequiredField(cluster, "package_md5sum"); err != nil {
        return nil, err
    }

    if cfgMap["jobs"] == nil {
        return nil, fmt.Errorf("`jobs` key does not exists.")
    }
    if reflect.TypeOf(cfgMap["jobs"]).Kind() != reflect.Map {
        return nil, fmt.Errorf("`jobs` key format is incorrect.")
    }

    jobsMap := cfgMap["jobs"].(map[interface{}]interface{})

    jobs := make(map[string]Job)
    for jobName := range jobsMap {
        if reflect.TypeOf(jobsMap[jobName]).Kind() != reflect.Map {
            return nil, fmt.Errorf("job `%s` format is incorrect.", jobName)
        }
        jobMap := jobsMap[jobName].(map[interface{}]interface{})
        if job, err2 := NewJob(jobName.(string), jobMap); err2 != nil {
            return nil, err2
        }else {
            jobs[jobName.(string)] = job
        }
    }
    svCfg.jobs = jobs
    return svCfg, nil
}

func (s *ServiceConfig) toShell(jobKey string) string {
    if _, ok := s.jobs[jobKey]; !ok {
        return ""
    }
    var buf []string
    buf = append(buf, s.javaHome)
    buf = append(buf, s.jobs[jobKey].toShell())
    return strings.Join(buf, SHELL_SEPARATOR)
}

func MergeYamlMap(m1 map[interface{}]interface{}, m2 map[interface{}]interface{}) map[interface{}]interface{} {
    if m2 == nil || reflect.TypeOf(m2).Kind() != reflect.Map {
        return m1
    }
    if m1 == nil || reflect.TypeOf(m1).Kind() != reflect.Map {
        return m2
    }

    for key := range m2 {
        value := m2[key]
        if value == nil {
            continue
        }
        if m1[key] == nil {
            m1[key] = value
            continue
        }
        switch reflect.TypeOf(value).Kind(){
        case reflect.Slice, reflect.Array:
            kind := reflect.TypeOf(m1[key]).Kind()
            if kind != reflect.Slice && kind != reflect.Array {
                panic("Type mismatch")
            }
            a1 := m1[key].([]interface{})
            a2 := m2[key].([]interface{})
            var a3 []interface{}
            for i := range a1 {
                a3 = append(a3, a1[i])
            }
            for i := range a2 {
                exist := false
                for j := range a1 {
                    if a1[j] == a2[i] {
                        exist = true
                    }
                }
                if exist {
                    continue
                }
                a3 = append(a3, a2[i])
            }
            m1[key] = a3
        case reflect.Map:
            if reflect.TypeOf(m1[key]).Kind() != reflect.Map {
                panic("Type mismatch")
            }
            m1[key] = MergeYamlMap(m1[key].(map[interface{}]interface{}), m2[key].(map[interface{}]interface{}))
        default:
            continue
        }
    }

    return m1
}
