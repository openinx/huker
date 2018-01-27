package huker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type matchFunc func(c *Cluster, input string) (string, error)

// format %{cluster.name}
func match0(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile("%{cluster.name}")
	return re.ReplaceAllString(input, c.clusterName), nil
}

// format %{namenode.0.host}
func match1(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile("%{([a-zA-Z_]+).([0-9]+).([a-zA-Z_]+)}")
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, taskIdStr, key := match[0], match[1], match[2]
		matchPatten := fmt.Sprintf("%%{%s.%s.%s}", jobName, taskIdStr, key)
		if _, ok := c.jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer. %s", matchPatten)
		}
		job := c.jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.attributes[key]; ok {
			input = strings.Replace(input, matchPatten, val, 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{name.0.base_port+1}
func match2(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile("%{([a-zA-Z_]+).([0-9]+).([a-zA-Z_]+)\\+([0-9]+)}")
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, taskIdStr, key, incrStr := match[0], match[1], match[2], match[3]
		matchPatten := fmt.Sprintf("%%{%s.%s.%s+%s}", jobName, taskIdStr, key, incrStr)
		if _, ok := c.jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		taskId, err := strconv.Atoi(taskIdStr)
		if err != nil {
			return "", fmt.Errorf("TaskId shoud be integer, %s", matchPatten)
		}
		job := c.jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.attributes[key]; ok {
			incr, _ := strconv.Atoi(incrStr)
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return "", err
			}
			input = strings.Replace(input, matchPatten, fmt.Sprintf("%d", valInt +incr), 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// format %{dependencies.0.zkServer.server_list}
func match3(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(`%{dependencies.([0-9]+).([a-zA-Z_]+).server_list}`)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		clusterIndexStr, jobName := match[0], match[1]
		matchPattern := fmt.Sprintf("%%{dependencies.%s.%s.server_list}", clusterIndexStr, jobName)
		clusterIndex, _ := strconv.Atoi(clusterIndexStr)
		if clusterIndex >= len(c.dependencies) {
			return "", fmt.Errorf("Cluster index exceeded. %s", matchPattern)
		}
		dep := c.dependencies[clusterIndex]
		job, ok := dep.jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, dep.clusterName)
		}
		var buf []string
		for _, host := range job.hosts {
			buf = append(buf, fmt.Sprintf("%s:%d", host.hostname, host.basePort))
		}
		input = strings.Replace(input, matchPattern, strings.Join(buf, ","), 1)
	}
	return input, nil
}

// format %{journalnode.server_list}
func match4(c *Cluster, input string) (string, error) {
	re := regexp.MustCompile(`%{([a-zA-Z_]+).server_list}`)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName := match[0]
		matchPattern := fmt.Sprintf("%%{%s.server_list}", jobName)
		job, ok := c.jobs[jobName]
		if !ok {
			return "", fmt.Errorf("Job %s does not exist in cluster: %s", jobName, c.clusterName)
		}
		var buf []string
		for _, host := range job.hosts {
			buf = append(buf, fmt.Sprintf("%s:%d", host.hostname, host.basePort))
		}
		input = strings.Replace(input, matchPattern, strings.Join(buf, ","), 1)
	}
	return input, nil
}

type matchHostFunc func(c *Cluster, taskId int, input string) (string, error)

// format %{namenode.x.base_port}
// TODO BUG: the taskId may not match with the job.
func match5(c *Cluster, taskId int, input string) (string, error) {
	re := regexp.MustCompile("%{([a-zA-Z_]+).x.([a-zA-Z_]+)}")
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, key := match[0], match[1]
		matchPatten := fmt.Sprintf("%%{%s.x.%s}", jobName, key)
		if _, ok := c.jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		job := c.jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.attributes[key]; ok {
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
	re := regexp.MustCompile("%{([a-zA-Z_]+).x.([a-zA-Z_]+)\\+([0-9]+)}")
	matches := re.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		match = match[1:]
		jobName, key, incrStr := match[0], match[1], match[2]
		matchPatten := fmt.Sprintf("%%{%s.x.%s+%s}", jobName, key, incrStr)
		if _, ok := c.jobs[jobName]; !ok {
			return "", fmt.Errorf("Invalid job name. %s", matchPatten)
		}
		job := c.jobs[jobName]
		host, ok := job.GetHost(taskId)
		if !ok {
			return "", fmt.Errorf("Task not found. %s", matchPatten)
		}
		if val, ok := host.attributes[key]; ok {
			incr, _ := strconv.Atoi(incrStr)
			valInt, err := strconv.Atoi(val)
			if err != nil {
				return "", err
			}
			input = strings.Replace(input, matchPatten, fmt.Sprintf("%d", valInt +incr), 1)
		} else {
			return "", fmt.Errorf("Attribute %s not exist. %s", key, matchPatten)
		}
	}
	return input, nil
}

// Render the %{<job>.<index>.<attribute>} and %{dependencies.<index>.<job>.server_list} of input string
// to value for the global cluster.
func GlobalRender(c *Cluster, input string) (string, error) {
	var err error
	for _, matchFun := range []matchFunc{
		match0, match1, match2, match3, match4,
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
