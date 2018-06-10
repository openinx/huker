package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	patternClusterName                    = "%{cluster\\.name}"
	patternJobAttribute                   = "%{([a-zA-Z0-9_]+)\\.([0-9]+)\\.([a-zA-Z0-9_]+)}"
	patternJobAttributeNumber             = "%{([a-zA-Z0-9_]+)\\.([0-9]+)\\.([a-zA-Z0-9_]+)\\+([0-9]+)}"
	patternDependenciesServerList         = "%{dependencies\\.([0-9]+)\\.([a-zA-Z0-9_]+)\\.server_list}"
	patternDependenciesJobAttribute       = "%{dependencies\\.([0-9]+)\\.([a-zA-Z0-9_]+)\\.([0-9]+)\\.([a-zA-Z0-9_]+)}"
	patternDependenciesJobAttributeNumber = "%{dependencies\\.([0-9]+)\\.([a-zA-Z0-9_]+)\\.([0-9]+)\\.([a-zA-Z0-9_]+)\\+([0-9]+)}"
	patternDependenciesClusterName        = "%{dependencies\\.([0-9]+)\\.cluster_name}"
	patternJobServerList                  = "%{([a-zA-Z0-9_]+)\\.server_list}"
	patternVarJobAttribute                = "%{([a-zA-Z0-9_]+)\\.x\\.([a-zA-Z0-9_]+)}"
	patternVarJobAttributeNumber          = "%{([a-zA-Z0-9_]+)\\.x\\.([a-zA-Z0-9_]+)\\+([0-9]+)}"
)

type matchFunc func(c *Cluster, input string) (string, error)

// format %{cluster.name}
func match0(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternClusterName)
	return re.ReplaceAllString(input, c.ClusterName), nil
}

// format %{namenode.0.host}
func match1(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternJobAttribute)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, taskIdStr, key := match[0], match[1], match[2]
		matchPatten := fmt.Sprintf("%%{%s.%s.%s}", jobName, taskIdStr, key)
		// It's possible that the format conflict with the format %{dependencies.0.cluster_name}
		if jobName == "dependencies" {
			continue
		}
		if _, ok := c.Jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer. %s", matchPatten)
		}
		job := c.Jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.Attributes[key]; ok {
			input = strings.Replace(input, matchPatten, val, 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{name.0.base_port+1}
func match2(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternJobAttributeNumber)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, taskIdStr, key, incrStr := match[0], match[1], match[2], match[3]
		matchPatten := fmt.Sprintf("%%{%s.%s.%s+%s}", jobName, taskIdStr, key, incrStr)
		if _, ok := c.Jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer, %s", matchPatten)
		}
		job := c.Jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.Attributes[key]; ok {
			incr, _ := strconv.Atoi(incrStr)
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return "", err
			}
			input = strings.Replace(input, matchPatten, fmt.Sprintf("%d", valInt+incr), 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{dependencies.0.zkServer.server_list}
func match3(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternDependenciesServerList)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		clusterIndexStr, jobName := match[0], match[1]
		matchPattern := fmt.Sprintf("%%{dependencies.%s.%s.server_list}", clusterIndexStr, jobName)
		clusterIndex, _ := strconv.Atoi(clusterIndexStr)
		if clusterIndex >= len(c.Dependencies) {
			return "", fmt.Errorf("Cluster index exceeded. %s", matchPattern)
		}
		dep := c.Dependencies[clusterIndex]
		job, ok := dep.Jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, dep.ClusterName)
		}
		var buf []string
		for _, host := range job.Hosts {
			buf = append(buf, fmt.Sprintf("%s:%d", host.Hostname, host.BasePort))
		}
		input = strings.Replace(input, matchPattern, strings.Join(buf, ","), 1)
	}
	return input, nil
}

// format %{journalnode.server_list}
func match4(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternJobServerList)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName := match[0]
		matchPattern := fmt.Sprintf("%%{%s.server_list}", jobName)
		job, ok := c.Jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, c.ClusterName)
		}
		var buf []string
		for _, host := range job.Hosts {
			buf = append(buf, fmt.Sprintf("%s:%d", host.Hostname, host.BasePort))
		}
		input = strings.Replace(input, matchPattern, strings.Join(buf, ","), 1)
	}
	return input, nil
}

type matchHostFunc func(c *Cluster, taskId int, input string) (string, error)

// format %{namenode.x.base_port}
// TODO BUG: the taskId may not match with the job.
func match5(c *Cluster, taskId int, input string) (string, error) {
	re := regexp.MustCompile(patternVarJobAttribute)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, key := match[0], match[1]
		matchPatten := fmt.Sprintf("%%{%s.x.%s}", jobName, key)
		if _, ok := c.Jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		job := c.Jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.Attributes[key]; ok {
			input = strings.Replace(input, matchPatten, val, 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{namenode.x.base_port+1}
// TODO BUG: the taskId may not match with the job.
func match6(c *Cluster, taskId int, input string) (string, error) {
	re := regexp.MustCompile(patternVarJobAttributeNumber)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, key, incrStr := match[0], match[1], match[2]
		matchPatten := fmt.Sprintf("%%{%s.x.%s+%s}", jobName, key, incrStr)
		if _, ok := c.Jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		job := c.Jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.Attributes[key]; ok {
			incr, _ := strconv.Atoi(incrStr)
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return "", err
			}
			input = strings.Replace(input, matchPatten, fmt.Sprintf("%d", valInt+incr), 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{dependencies.0.zkServer.0.host}
func match7(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternDependenciesJobAttribute)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		clusterIndexStr, jobName, taskIdStr, key := match[0], match[1], match[2], match[3]
		matchPattern := fmt.Sprintf("%%{dependencies.%s.%s.%s.%s}", clusterIndexStr, jobName, taskIdStr, key)
		clusterIndex, _ := strconv.Atoi(clusterIndexStr)
		if clusterIndex >= len(c.Dependencies) {
			return "", fmt.Errorf("Cluster index exceeded. %s", matchPattern)
		}
		dep := c.Dependencies[clusterIndex]
		job, ok := dep.Jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, dep.ClusterName)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer. %s", matchPattern)
		}
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s, taskId: %d, job: %v", matchPattern, taskId, job)
		}
		if val, ok := host.Attributes[key]; ok {
			input = strings.Replace(input, matchPattern, val, 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPattern)
		}
	}
	return input, nil
}

// format %{dependencies.0.zkServer.0.base_port+1}
func match8(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternDependenciesJobAttributeNumber)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		clusterIndexStr, jobName, taskIdStr, key, incrStr := match[0], match[1], match[2], match[3], match[4]
		matchPattern := fmt.Sprintf("%%{dependencies.%s.%s.%s.%s+%s}", clusterIndexStr, jobName, taskIdStr, key, incrStr)
		clusterIndex, _ := strconv.Atoi(clusterIndexStr)
		if clusterIndex >= len(c.Dependencies) {
			return "", fmt.Errorf("Cluster index exceeded. %s", matchPattern)
		}
		dep := c.Dependencies[clusterIndex]
		job, ok := dep.Jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, dep.ClusterName)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer, %s", matchPattern)
		}
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPattern)
		}
		if val, ok := host.Attributes[key]; ok {
			incr, _ := strconv.Atoi(incrStr)
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return "", err
			}
			input = strings.Replace(input, matchPattern, fmt.Sprintf("%d", valInt+incr), 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPattern)
		}
	}
	return input, nil
}

// format %{dependencies.0.cluster_name}
func match9(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(patternDependenciesClusterName)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		clusterIndexStr := match[0]
		matchPattern := fmt.Sprintf("%%{dependencies.%s.cluster_name}", clusterIndexStr)
		clusterIndex, _ := strconv.Atoi(clusterIndexStr)
		if clusterIndex >= len(c.Dependencies) {
			return "", fmt.Errorf("Cluster index exceeded. %s", matchPattern)
		}
		dep := c.Dependencies[clusterIndex]
		input = strings.Replace(input, matchPattern, dep.ClusterName, 1)
	}
	return input, nil
}

// Render the %{<job>.<index>.<attribute>} and %{dependencies.<index>.<job>.server_list} of input string
// to value for the global cluster.
func GlobalRender(c *Cluster, input string) (string, error) {
	var err error
	for _, matchFun := range []matchFunc{
		match0, match1, match2, match3, match4, match7, match8, match9,
	} {
		input, err = matchFun(c, input)
		if err != nil {
			return "", err
		}
	}
	return input, nil
}

// Render the %{<job>.x.<attribute>} of input string to value for the specific host.
func HostRender(c *Cluster, taskId int, input string) (string, error) {
	var err error
	for _, matchFun := range []matchHostFunc{
		match5, match6,
	} {
		input, err = matchFun(c, taskId, input)
		if err != nil {
			return "", err
		}
	}
	return input, nil
}
