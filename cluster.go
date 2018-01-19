package huker

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"io/ioutil"
)

type Cluster struct {
	baseConfig    string
	clusterName   string
	javaHome      string
	packageName   string
	packageMd5sum string
	jobs          map[string]*Job
	dependencies  []*Cluster
}

func getRequiredField(m map[interface{}]interface{}, key string) (string, error) {
	if obj, ok := m[key]; !ok || obj == nil {
		return "", fmt.Errorf("Required key `%s` does not exist in config file.", key)
	} else if !IsStringType(obj) {
		return "", fmt.Errorf("`%s` should be a string: %v", key, m)
	} else {
		return obj.(string), nil
	}
}

func NewCluster(yamlConfigs []string, e *EnvVariables) (*Cluster, error) {
	cfgMap, err := mergeYamlConfigs(yamlConfigs)
	if err != nil {
		return nil, err
	}
	c := &Cluster{
		jobs: make(map[string]*Job),
	}

	// Read `base` section.
	if obj, ok := cfgMap["base"]; ok && obj != nil {
		if !IsStringType(obj) {
			return nil, fmt.Errorf("Invalid cluster config, `base` should be a string path. %v", obj)
		}
		c.baseConfig = obj.(string)
	}

	// Read `cluster` section.
	var clusterMap map[interface{}]interface{}
	if obj, ok := cfgMap["cluster"]; !ok || obj == nil {
		return nil, fmt.Errorf("`cluster` section does not exists.")
	} else if !IsMapType(obj) {
		return nil, fmt.Errorf("`cluster` section shoud be a map. %v", obj)
	} else {
		clusterMap = obj.(map[interface{}]interface{})
	}
	if c.clusterName, err = getRequiredField(clusterMap, "cluster_name"); err != nil {
		return nil, err
	}
	if c.javaHome, err = getRequiredField(clusterMap, "java_home"); err != nil {
		return nil, err
	}
	if c.packageName, err = getRequiredField(clusterMap, "package_name"); err != nil {
		return nil, err
	}
	if c.packageMd5sum, err = getRequiredField(clusterMap, "package_md5sum"); err != nil {
		return nil, err
	}
	// Read `dependencies` section.
	dependencies, err := ParseStringArray(clusterMap["dependencies"])
	if err == nil && len(dependencies) > 0 {
		for _, dep := range dependencies {
			depCluster, err := LoadClusterConfig(dep, e)
			if err != nil {
				return nil, err
			}
			c.dependencies = append(c.dependencies, depCluster)
		}
	}

	// Read `jobs` section.
	var jobsMap map[interface{}]interface{}
	if obj, ok := cfgMap["jobs"]; !ok || obj == nil {
		return nil, fmt.Errorf("`jobs` section does not exists.")
	} else if !IsMapType(obj) {
		return nil, fmt.Errorf("`jobs` section shoud be a map. %v", obj)
	} else {
		jobsMap = obj.(map[interface{}]interface{})
	}
	for jobName, jobMap := range jobsMap {
		if !IsStringType(jobName) {
			return nil, fmt.Errorf("Job name should be a string. %v", jobName)
		}
		if !IsMapType(jobMap) {
			return nil, fmt.Errorf("Job `%s` section should be a map, %v", jobName, jobMap)
		}
		if job, err := NewJob(jobName.(string), jobMap.(map[interface{}]interface{})); err != nil {
			return nil, err
		} else {
			c.jobs[jobName.(string)] = job
		}
	}

	// Merge job with its parent job. Be careful that we only allow one layer of inheritance.
	// The case A -> B -> C is not allowed.
	newJobs := make(map[string]*Job)
	for jobName, job := range c.jobs {
		if parentJob, ok := c.jobs[job.superJob]; ok && parentJob != nil {
			if job, err = job.mergeWith(parentJob); err != nil {
				return nil, err
			}
		}
		newJobs[jobName] = job
	}
	c.jobs = newJobs

	return c, nil
}

func (s *Cluster) toShell(jobKey string) []string {
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

func (c *Cluster) RenderConfigFiles(job *Job, taskId int) (map[string]string, error) {
	var ok bool
	// var host *Host
	if job, ok = c.jobs[job.jobName]; !ok {
		return nil, fmt.Errorf("Job with name `%s` not exist in cluster %s", job.jobName, c.clusterName)
	}
	if _, ok = job.GetHost(taskId); !ok {
		return nil, fmt.Errorf("TaskId `%s` not exist in job `%s` for cluster `%s`", taskId, job.jobName, c.clusterName)
	}

	cfgMap := make(map[string]string)
	for fname, cfg := range job.configFiles {
		newCfg, err := GlobalRender(c, cfg.toString())
		if err != nil {
			return nil, err
		}
		newCfg, err = HostRender(c, taskId, newCfg)
		if err != nil {
			return nil, err
		}
		cfgMap[fname] = newCfg
	}

	return cfgMap, nil
}

func LoadClusterConfig(yamlCfgPath string, e *EnvVariables) (*Cluster, error) {
	if cfgContents, err := readYamlConfig(yamlCfgPath, e); err != nil {
		return nil, err
	} else {
		return NewCluster(cfgContents, e)
	}
}

func readYamlConfig(yamlCfgPath string, e *EnvVariables) ([]string, error) {
	data, err := ioutil.ReadFile(yamlCfgPath)
	if err != nil {
		return []string{}, err
	}
	// Render the template
	if dataStr, err := e.RenderTemplate(string(data)); err != nil {
		return []string{}, err
	} else {
		data = []byte(dataStr)
	}

	cfgMap := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(data, &cfgMap); err != nil {
		return []string{}, err
	}

	// Continue to read the `base` configuration files.
	strings := []string{string(data)}
	if obj, ok := cfgMap["base"]; !ok || obj == nil {
		return strings, nil
	} else if !IsStringType(obj) {
		return []string{}, fmt.Errorf("`base` section shoud be a string: %v", obj)
	} else {
		baseConfigs, err := readYamlConfig(obj.(string), e)
		if err != nil {
			return []string{}, err
		}
		return append(strings, baseConfigs...), nil
	}
}

// There are many .yaml configurations, for example yamlConfigs[0], yamlConfigs[1], ..., which the first conf will merge
// with the second one (the first win if encounter a same key), the second will merge with the third one.
func mergeYamlConfigs(yamlConfigs []string) (map[interface{}]interface{}, error) {
	if len(yamlConfigs) == 0 {
		return nil, fmt.Errorf("No yaml config provided.")
	}
	yamlMaps := make([]map[interface{}]interface{}, len(yamlConfigs))
	for i := range yamlConfigs {
		yamlMaps[i] = map[interface{}]interface{}{}
		if err := yaml.Unmarshal([]byte(yamlConfigs[i]), &yamlMaps[i]); err != nil {
			return nil, fmt.Errorf("Unmarshal yaml error, config: %s, cause: %v", yamlConfigs[i], err)
		}
	}
	ret := yamlMaps[0]
	for i := 1; i < len(yamlMaps); i++ {
		ret = MergeMap(ret, yamlMaps[i])
	}
	return ret, nil
}
