package huker

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/qiniu/log"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
)

/********************************** ConfigFile Implementation *****************************/
type ConfigFile interface {
	mergeWith(c ConfigFile) ConfigFile
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
		cfgName:   cfgName,
		keyValues: keyValues,
	}
}

func (c INIConfigFile) mergeWith(other ConfigFile) ConfigFile {
	cMap := c.toKeyValue()
	oMap := other.toKeyValue()
	// If key exist in both cMap and oMap, then use value of cMap.
	for key, val := range cMap {
		oMap[key] = val
	}
	// convert oMap to []string
	keyValues := []string{}
	for key, val := range oMap {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, val))
	}
	c.keyValues = keyValues
	return c
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
		cfgName:   cfgName,
		keyValues: keyValues,
	}
}

func (c XMLConfigFile) mergeWith(other ConfigFile) ConfigFile {
	cMap := c.toKeyValue()
	oMap := other.toKeyValue()
	// If key exist in both cMap and oMap, the use value of cMap.
	for key, val := range cMap {
		oMap[key] = val
	}
	// convert oMap to []string
	keyValues := []string{}
	for key, val := range oMap {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, val))
	}
	c.keyValues = keyValues
	return c
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

type PlainConfigFile struct {
	cfgName string
	lines   []string
}

func NewPlainConfigFile(cfgName string, lines []string) PlainConfigFile {
	return PlainConfigFile{
		cfgName: cfgName,
		lines:   lines,
	}
}

func (c PlainConfigFile) mergeWith(other ConfigFile) ConfigFile {
	for _, line := range other.toKeyValue() {
		c.lines = append(c.lines, line)
	}
	return c
}

func (c PlainConfigFile) toString() string {
	var buf []string
	for _, line := range c.lines {
		buf = append(buf, line)
	}
	return strings.Join(buf, "\n")
}

func (c PlainConfigFile) toKeyValue() map[string]string {
	ret := make(map[string]string)
	for i, line := range c.lines {
		ret[strconv.Itoa(i)] = line
	}
	return ret
}

func (c PlainConfigFile) getConfigName() string {
	return c.cfgName
}

/********************************** ServiceConfig Implementation *****************************/

type MainEntry struct {
	javaClass string
	extraArgs string
}

func (m MainEntry) toShell() []string {
	return []string{m.javaClass, m.extraArgs}
}

type HostInfo struct {
	hostName    string
	taskId      string
	hostPort    string
	configFiles map[string]ConfigFile
}

func NewHostInfo(hostKey string, configFiles []ConfigFile) HostInfo {
	parts := strings.Split(hostKey, "/")
	hostName := parts[0]
	hostPort := "9001" // default agent port
	taskId := "1"      // default task Id
	for _, s := range parts {
		if strings.HasPrefix(s, "port=") {
			hostPort = strings.TrimPrefix(s, "port=")
		}
		if strings.HasPrefix(s, "id=") {
			taskId = strings.TrimPrefix(s, "id=")
		}
	}
	host := HostInfo{hostName: hostName, taskId: taskId, hostPort: hostPort}

	host.configFiles = make(map[string]ConfigFile)
	for _, cf := range configFiles {
		host.configFiles[cf.getConfigName()] = cf
	}
	return host
}

func (h HostInfo) toHttpAddress() string {
	return fmt.Sprintf("http://%s:%s", h.hostName, h.hostPort)
}

func (h HostInfo) toKey() string {
	return fmt.Sprintf("%s/port=%s/id=%s", h.hostName, h.hostPort, h.taskId)
}

func (h HostInfo) toConfigMap() map[string]string {
	cfgMap := make(map[string]string)
	for fname, cfgFile := range h.configFiles {
		cfgMap[fname] = cfgFile.toString()
	}
	return cfgMap
}

func (h HostInfo) mergeWith(configFiles map[string]ConfigFile) {
	for fname, cfg := range configFiles {
		if _, ok := h.configFiles[fname]; ok {
			newCfg := h.configFiles[fname].mergeWith(cfg)
			h.configFiles[fname] = newCfg
		} else {
			h.configFiles[fname] = cfg
		}
	}
}

type Job struct {
	jobName       string
	hosts         []HostInfo
	jvmOpts       []string
	jvmProperties []string
	classpath     []string
	configFiles   map[string]ConfigFile
	mainEntry     MainEntry
}

func (job Job) toShell() []string {
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

	for _, s := range job.mainEntry.toShell() {
		buf = append(buf, s)
	}
	return buf
}

func (job Job) toConfigMap() map[string]string {
	cfgMap := make(map[string]string)
	for cfgKey, cfgFile := range job.configFiles {
		cfgMap[cfgKey] = cfgFile.toString()
	}
	return cfgMap
}

type ServiceConfig struct {
	baseConfig    string
	clusterName   string
	javaHome      string
	packageName   string
	packageMd5sum string
	jobs          map[string]Job
}

func readYamlConfig(yamlCfgPath string, e *EnvVariables) ([]string, error) {
	data, err := ioutil.ReadFile(yamlCfgPath)
	if err != nil {
		return []string{}, err
	}
	// Render the template
	dataStr, err2 := e.RenderTemplate(string(data))
	if err2 != nil {
		return []string{}, err2
	}
	data = []byte(dataStr)

	cfgMap := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(data, &cfgMap); err != nil {
		return []string{}, err
	}
	strings := []string{string(data)}
	baseCfg := cfgMap["base"]
	if baseCfg == nil {
		return strings, nil
	} else {
		if reflect.TypeOf(baseCfg).Kind() != reflect.String {
			return []string{}, fmt.Errorf("base configuration path is not a string: %v", baseCfg)
		}
		baseConfigs, err2 := readYamlConfig(baseCfg.(string), e)
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
			typeOItem := reflect.TypeOf(array[i])
			if typeOItem.Kind() == reflect.String {
				strings = append(strings, array[i].(string))
			} else if typeOItem.Kind() == reflect.Int {
				strings = append(strings, strconv.Itoa(array[i].(int)))
			} else {
				return []string{}, fmt.Errorf("Invalid type: %v, %v", typeOItem.Kind(), array[i])
			}
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
	} else if strings.HasSuffix(cfgName, ".xml") {
		return NewXMLConfigFile(cfgName, keyValues), nil
	} else if !strings.Contains(cfgName, ".") || strings.HasSuffix(cfgName, ".txt") {
		return NewPlainConfigFile(cfgName, keyValues), nil
	} else {
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
		} else {
			if cf, err := parseConfigFile(cfgName, keyValues); err != nil {
				return nil, err
			} else {
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
	var err error
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
		} else {
			job.configFiles = make(map[string]ConfigFile)
			for i := range confFiles {
				job.configFiles[confFiles[i].getConfigName()] = confFiles[i]
			}
		}
	}

	if jobMap["hosts"] != nil {
		if reflect.TypeOf(jobMap["hosts"]).Kind() != reflect.Map {
			return Job{}, fmt.Errorf("`hosts` part of job `%s` is not a map", jobName)
		}
		hostsMap := jobMap["hosts"].(map[interface{}]interface{})

		hosts := []HostInfo{}
		for hostKey, hostObj := range hostsMap {
			if reflect.TypeOf(hostObj).Kind() != reflect.Map {
				return Job{}, fmt.Errorf("`config` part of host `%s` in job `%s` is not a map", hostKey, jobName)
			}

			hostMap := hostObj.(map[interface{}]interface{})
			if confFiles, err2 := parseConfigFileArray(hostMap["config"]); err2 != nil {
				return Job{}, err2
			} else {
				hostInfo := NewHostInfo(hostKey.(string), confFiles)
				hostInfo.mergeWith(job.configFiles)
				hosts = append(hosts, hostInfo)
			}
		}
		job.hosts = hosts
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

func LoadServiceConfig(yamlCfgPath string, e *EnvVariables) (*ServiceConfig, error) {
	if cfgContents, err := readYamlConfig(yamlCfgPath, e); err != nil {
		return nil, err
	} else {
		return NewServiceConfig(cfgContents)
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
		} else {
			jobs[jobName.(string)] = job
		}
	}
	svCfg.jobs = jobs
	return svCfg, nil
}

func (s *ServiceConfig) toShell(jobKey string) []string {
	if _, ok := s.jobs[jobKey]; !ok {
		return []string{}
	}
	var buf []string
	buf = append(buf, s.javaHome)
	for _, arg := range s.jobs[jobKey].toShell() {
		buf = append(buf, arg)
	}
	return buf
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
		switch reflect.TypeOf(value).Kind() {
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
