package huker

import (
	"bytes"
	"fmt"
	"github.com/qiniu/log"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
)

const (
	DEFAULT_SUPERVISOR_PORT = 9001
	DEFAULT_BASE_PORT       = 10000
	DEFAULT_TASK_ID         = 0
)

type MainEntry struct {
	JavaClass string
	ExtraArgs string
}

func parseMainEntry(s interface{}) (*MainEntry, error) {
	if !IsMapType(s) {
		return nil, fmt.Errorf("Invalid main_entry, not a map")
	}
	mainEntry := &MainEntry{}
	meMap := s.(map[interface{}]interface{})
	if obj, ok := meMap["java_class"]; ok && obj != nil {
		if !IsStringType(obj) {
			return nil, fmt.Errorf("Invalid main entry, java_class is not a string. %v", s)
		}
		mainEntry.JavaClass = strings.Trim(obj.(string), " ")
	}
	if obj, ok := meMap["extra_args"]; ok && obj != nil {
		if !IsStringType(obj) {
			return nil, fmt.Errorf("Invalid main_entry, extra_args is not a string. %v", s)
		}
		mainEntry.ExtraArgs = meMap["extra_args"].(string)
	}
	return mainEntry, nil
}

func (m *MainEntry) toShell() []string {
	var buf []string
	if len(m.JavaClass) > 0 {
		buf = append(buf, m.JavaClass)
	}
	// TODO need to consider tab ?
	for _, arg := range strings.Split(m.ExtraArgs, " ") {
		if len(arg) > 0 {
			buf = append(buf, arg)
		}
	}
	return buf
}

type Host struct {
	Hostname       string
	TaskId         int
	SupervisorPort int
	BasePort       int
	Attributes     map[string]string
}

func NewHost(hostKey string) (*Host, error) {
	host := &Host{
		Hostname:       "",
		TaskId:         DEFAULT_TASK_ID,
		SupervisorPort: DEFAULT_SUPERVISOR_PORT,
		BasePort:       DEFAULT_BASE_PORT,
		Attributes:     make(map[string]string),
	}

	var err error
	splits := strings.Split(hostKey, "/")
	if len(splits) <= 0 {
		return nil, fmt.Errorf("Host should not be empty")
	}
	hostAndPort := strings.Split(splits[0], ":")
	if len(hostAndPort) != 2 {
		return nil, fmt.Errorf("Invalid supervisor address: %s . should be format: <hostname>:<port>", splits[0])
	}
	host.Hostname = hostAndPort[0]
	host.SupervisorPort, err = strconv.Atoi(hostAndPort[1])
	if err != nil {
		return nil, fmt.Errorf("Invalid supervisor address: %s, port should be integer.", splits[0])
	}
	host.Attributes["host"] = hostAndPort[0]
	host.Attributes["port"] = hostAndPort[1]

	for _, split := range splits[1:] {
		keyValues := strings.Split(split, "=")
		if len(keyValues) != 2 {
			return nil, fmt.Errorf("Invalid key-value pair: %s . shoud be format like: <key>=<value>.", hostKey)
		}
		host.Attributes[keyValues[0]] = keyValues[1]
		if keyValues[0] == "id" {
			host.TaskId, err = strconv.Atoi(keyValues[1])
			if err != nil {
				return nil, err
			}
			if host.TaskId < 0 {
				return nil, fmt.Errorf("Invalid taskId, shouldn't be negative. %s", hostKey)
			}
		}
		if keyValues[0] == "base_port" {
			host.BasePort, err = strconv.Atoi(keyValues[1])
			if err != nil {
				return nil, err
			}
			if host.BasePort <= 0 {
				return nil, fmt.Errorf("Invalid basePort, should be positive integer. %s", hostKey)
			}
		}
	}

	return host, nil
}

func (h *Host) ToHttpAddress() string {
	return fmt.Sprintf("http://%s:%d", h.Hostname, h.SupervisorPort)
}

func (h *Host) ToKey() string {
	return fmt.Sprintf("%s:%d/id=%d", h.Hostname, h.SupervisorPort, h.TaskId)
}

func (h *Host) toConfigMap() map[string]string {
	// TODO Render the config map of job's config files with variables of this Host.
	return nil
}

func mergeConfigFiles(this, other map[string]ConfigFile) map[string]ConfigFile {
	for fname, cfg := range other {
		if _, ok := this[fname]; ok {
			this[fname] = this[fname].mergeWith(cfg)
		} else {
			this[fname] = cfg
		}
	}
	return this
}

type Job struct {
	JobName       string
	SuperJob      string
	Hosts         []*Host
	JvmOpts       []string
	JvmProperties []string
	Classpath     []string
	MainEntry     *MainEntry
	ConfigFiles   map[string]ConfigFile
	Hooks         map[string]string
}

func NewJob(jobName string, jobMap map[interface{}]interface{}) (*Job, error) {
	job := &Job{
		JobName:     jobName,
		SuperJob:    "", // No super job by default.
		Hosts:       []*Host{},
		MainEntry:   &MainEntry{},
		ConfigFiles: make(map[string]ConfigFile),
		Hooks:       make(map[string]string),
	}
	var err error
	if obj, ok := jobMap["super_job"]; ok && obj != nil {
		if !IsStringType(obj) {
			return nil, fmt.Errorf("`super_job` field in job `%s` should be a string, now: %v", jobName, obj)
		}
		job.SuperJob = obj.(string)
	}
	if obj, ok := jobMap["jvm_opts"]; ok && obj != nil {
		if job.JvmOpts, err = ParseStringArray(obj); err != nil {
			return nil, err
		}
	}
	if obj, ok := jobMap["jvm_properties"]; ok && obj != nil {
		if job.JvmProperties, err = ParseStringArray(obj); err != nil {
			return nil, err
		}
	}
	if obj, ok := jobMap["classpath"]; ok && obj != nil {
		if job.Classpath, err = ParseStringArray(obj); err != nil {
			return nil, err
		}
	}
	if obj, ok := jobMap["config"]; ok && obj != nil {
		if confFiles, err := parseConfigFileArray(obj); err != nil {
			return nil, err
		} else {
			job.ConfigFiles = make(map[string]ConfigFile)
			for i := range confFiles {
				job.ConfigFiles[confFiles[i].GetConfigName()] = confFiles[i]
			}
		}
	}

	if obj, ok := jobMap["hosts"]; ok && obj != nil {
		hostKeys, err := ParseStringArray(obj)
		if err != nil {
			return nil, err
		}
		hosts := []*Host{}
		for _, hostKey := range hostKeys {
			host, err := NewHost(hostKey)
			if err != nil {
				return nil, err
			}
			hosts = append(hosts, host)
		}
		// Sort by taskId increase.
		sort.Slice(hosts, func(i, j int) bool {
			return hosts[i].TaskId < hosts[j].TaskId
		})
		job.Hosts = hosts
	}

	if jobMap["main_entry"] != nil {
		if job.MainEntry, err = parseMainEntry(jobMap["main_entry"]); err != nil {
			return nil, err
		}
	}

	if obj, ok := jobMap["hooks"]; ok && obj != nil && IsMapType(obj) {
		hooksMap := obj.(map[interface{}]interface{})
		for hookKey, hookPath := range hooksMap {
			if IsStringType(hookKey) && IsStringType(hookPath) {
				data, err := ioutil.ReadFile(hookPath.(string))
				if err != nil {
					return nil, err
				}
				job.Hooks[hookKey.(string)] = string(data)
			}
		}
	}
	return job, nil
}

func (job *Job) toShell() []string {
	var buf []string
	for i := range job.JvmOpts {
		jvmOpt := strings.TrimSpace(job.JvmOpts[i])
		if len(jvmOpt) > 0 {
			buf = append(buf, jvmOpt)
		}
	}
	for i := range job.JvmProperties {
		jvmPro := strings.TrimSpace(job.JvmProperties[i])
		if len(jvmPro) > 0 {
			buf = append(buf, fmt.Sprintf("-D%s", jvmPro))
		}
	}

	var classpath []string
	for i := range job.Classpath {
		cp := strings.TrimSpace(job.Classpath[i])
		if len(cp) > 0 {
			classpath = append(classpath, cp)
		}
	}
	if len(classpath) > 0 {
		buf = append(buf, "-cp")
		buf = append(buf, strings.Join(classpath, ":"))
	}

	for _, s := range job.MainEntry.toShell() {
		buf = append(buf, s)
	}
	return buf
}

func (job *Job) toConfigMap() map[string]string {
	cfgMap := make(map[string]string)
	for cfgKey, cfgFile := range job.ConfigFiles {
		cfgMap[cfgKey] = cfgFile.ToString()
	}
	return cfgMap
}

func (job *Job) mergeWith(other *Job) (*Job, error) {
	if job.SuperJob != other.JobName {
		return nil, fmt.Errorf("job `%s` is not inherited from job `%s`", job.JobName, other.JobName)
	}

	// Merge array a with array b. if exist in both a and b, then use item in a.
	mergeStringArray := func(a, b []string) []string {
		for k := range b {
			if !StringSliceContains(a, b[k]) {
				a = append(a, b[k])
			}
		}
		return a
	}

	// merge jvm opts
	job.JvmOpts = mergeStringArray(job.JvmOpts, other.JvmOpts)

	// merge jvm properties.
	job.JvmProperties = mergeStringArray(job.JvmProperties, other.JvmProperties)

	// merge jvm classpath
	job.Classpath = mergeStringArray(job.Classpath, other.Classpath)

	// merge config files
	job.ConfigFiles = mergeConfigFiles(job.ConfigFiles, other.ConfigFiles)
	return job, nil
}

func (job *Job) GetHost(taskId int) (*Host, bool) {
	for _, host := range job.Hosts {
		if host.TaskId == taskId {
			return host, true
		}
	}
	return nil, false
}

func parseConfigFileArray(obj interface{}) ([]ConfigFile, error) {
	if !IsMapType(obj) {
		return nil, fmt.Errorf("Invalid config, shoud be a map. %v", obj)
	}
	cfMap := obj.(map[interface{}]interface{})
	var cfgFiles []ConfigFile
	for key := range cfMap {
		cfgName := key.(string)
		if cfMap[key] == nil {
			log.Warnf("Configuration file has no key-value pairs. %s", key)
			continue
		}
		if keyValues, err := ParseStringArray(cfMap[key]); err != nil {
			return nil, err
		} else {
			if cf, err := ParseConfigFile(cfgName, keyValues); err != nil {
				return nil, err
			} else {
				cfgFiles = append(cfgFiles, cf)
			}
		}
	}
	return cfgFiles, nil
}

func ParseStringArray(obj interface{}) ([]string, error) {
	if IsArrayType(obj) || IsSliceType(obj) {
		array := obj.([]interface{})
		var strings []string
		for i := range array {
			if IsStringType(array[i]) {
				strings = append(strings, array[i].(string))
			} else if IsIntegerType(array[i]) {
				strings = append(strings, strconv.Itoa(array[i].(int)))
			} else {
				return nil, fmt.Errorf("Neither string nor int type. %v, origin: %v", array[i], obj)
			}
		}
		return strings, nil
	} else if IsMapType(obj) {
		mapObj := obj.(map[interface{}]interface{})
		if fileObj, ok := mapObj["file"]; ok && fileObj != nil {
			if IsStringType(fileObj) {
				fileBytes, err := ioutil.ReadFile(fileObj.(string))
				if err != nil {
					return nil, err
				}
				buf := bytes.NewBuffer(fileBytes)
				var results []string
				for buf.Len() > 0 {
					line, err := buf.ReadString(byte('\n'))
					if err != nil {
						return nil, fmt.Errorf("Read line failed, line: %s, %v", line, err)
					}
					line = strings.Trim(strings.Trim(line, "\n"), " ")
					if strings.HasPrefix(line, "#") {
						continue
					}
					if len(line) > 0 {
						results = append(results, line)
					}
				}
				return results, nil
			} else {
				return nil, fmt.Errorf("`file` part is not a string, %v", fileObj)
			}
		} else {
			return nil, fmt.Errorf("`file` part does not exist. %v", mapObj)
		}
	}
	return nil, fmt.Errorf("Neither array/slice nor map type, content: %v", obj)
}
