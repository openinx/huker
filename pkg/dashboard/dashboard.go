package pkg

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/openinx/huker"
	"github.com/qiniu/log"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	StatusRunning      = huker.StatusRunning
	StatusStopped      = huker.StatusStopped
	StatusNotBootstrap = "NotBootstrap"
	StatusUnknown      = "Unknown"
)

type Dashboard struct {
	Port          int
	srv           *http.Server
	hukerJob      huker.HukerJob
	refreshTicker *time.Ticker
	quit          chan int
	clusters      []*huker.Cluster
}

// Create a new supervisor agent.
func NewDashboard(port int) (*Dashboard, error) {
	hukerJob, err := huker.NewDefaultHukerJob()
	if err != nil {
		return nil, err
	}
	d := &Dashboard{
		Port: port,
		srv: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		hukerJob:      hukerJob,
		refreshTicker: time.NewTicker(5 * time.Second),
		quit:          make(chan int),
		clusters:      make([]*huker.Cluster, 0),
	}
	return d, nil
}

func (d *Dashboard) hIndex(w http.ResponseWriter, r *http.Request) {
	body, err := RenderTemplate("site/overview.html", "site/base.html", map[string]interface{}{}, nil)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Internal error: %v", err)))
		return
	}
	w.Write([]byte(body))
}

func (d *Dashboard) hList(w http.ResponseWriter, r *http.Request) {
	project := mux.Vars(r)["project"]
	clusters, err := d.hukerJob.List()
	if err != nil {
		log.Stack(err)
		w.Write([]byte(err.Error()))
		return
	}
	var projectClusters []*huker.Cluster
	for i := 0; i < len(clusters); i++ {
		if clusters[i].Project == project {
			projectClusters = append(projectClusters, clusters[i])
		}
	}

	body, err := RenderTemplate("site/list-cluster.html", "site/base.html", map[string]interface{}{
		"project":  project,
		"clusters": projectClusters}, nil)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Internal error: %v", err)))
		return
	}
	w.Write([]byte(body))
}

func (d *Dashboard) hDetail(w http.ResponseWriter, r *http.Request) {
	project := mux.Vars(r)["project"]
	clusterName := mux.Vars(r)["cluster"]
	var cluster *huker.Cluster
	for i := 0; i < len(d.clusters); i++ {
		if d.clusters[i].Project == project && d.clusters[i].ClusterName == clusterName {
			cluster = d.clusters[i]
		}
	}
	if cluster == nil {
		w.Write([]byte(fmt.Sprintf("cluster not found. project:%s, cluster:%s", project, clusterName)))
		return
	}

	body, err := RenderTemplate("site/detail.html", "site/base.html", map[string]interface{}{
		"cluster": cluster,
	}, template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	})
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Internal error: %v", err)))
		return
	}
	w.Write([]byte(body))
}

func (d *Dashboard) hConfig(w http.ResponseWriter, r *http.Request) {
	project := mux.Vars(r)["project"]
	clusterName := mux.Vars(r)["cluster"]
	jobName := mux.Vars(r)["job"]
	taskId, err := strconv.Atoi(mux.Vars(r)["task_id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("task_id should be a integer, instead of %s", mux.Vars(r)["task_id"])))
		return
	}

	var cluster *huker.Cluster
	for i := 0; i < len(d.clusters); i++ {
		if d.clusters[i].Project == project && d.clusters[i].ClusterName == clusterName {
			cluster = d.clusters[i]
		}
	}
	if cluster == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Cluster not found. project:%s, cluster:%s", project, clusterName)))
		return
	}
	job := cluster.Jobs[jobName]
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Job not found. project:%s, cluster:%s, job:%s", project, clusterName, jobName)))
		return
	}
	configMap, err2 := cluster.RenderConfigFiles(job, taskId, false)
	if err2 != nil {
		log.Stack(err2)
		w.Write([]byte(fmt.Sprintf("Render configuration files failed, project:%s, cluster:%s, job:%s,task:%d, err: %v",
			project, clusterName, jobName, taskId, err2)))
		return
	}

	isFirst := 1
	body, err := RenderTemplate("site/config.html", "site/base.html", map[string]interface{}{
		"cluster": cluster,
		"Job":     jobName,
		"TaskId":  strconv.Itoa(taskId),
		"config":  configMap,
	}, template.FuncMap{
		"checkIsFirst": func() int {
			if isFirst == 1 {
				isFirst = 0
				return 1
			} else {
				return 0
			}
		}, "reset": func() int {
			isFirst = 1
			return isFirst
		}, "transToId": func(s string) string {
			s = strings.Replace(s, ".", "_", -1)
			s = strings.Replace(s, "-", "_", -1)
			s = strings.Replace(s, "/", "_", -1)
			return s
		},
	})
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Internal error: %v", err)))
		return
	}
	w.Write([]byte(body))
}

func (d *Dashboard) hStaticFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "site/static/"+mux.Vars(r)["filename"])
}

func (d *Dashboard) hWebApi(w http.ResponseWriter, r *http.Request) {
	action := mux.Vars(r)["action"]
	project := mux.Vars(r)["project"]
	cluster := mux.Vars(r)["cluster"]
	job := mux.Vars(r)["job"]
	taskId, _ := strconv.Atoi(mux.Vars(r)["task_id"])

	var taskResults []huker.TaskResult
	var err error
	switch action {
	case "bootstrap":
		taskResults, err = d.hukerJob.Bootstrap(project, cluster, job, taskId)
	case "start":
		taskResults, err = d.hukerJob.Start(project, cluster, job, taskId)
	case "stop":
		taskResults, err = d.hukerJob.Stop(project, cluster, job, taskId)
	case "restart":
		taskResults, err = d.hukerJob.Restart(project, cluster, job, taskId)
	case "rolling_update":
		taskResults, err = d.hukerJob.RollingUpdate(project, cluster, job, taskId)
	case "cleanup":
		taskResults, err = d.hukerJob.Cleanup(project, cluster, job, taskId)
	default:
		panic("Unsupported action: " + action)
	}
	if err != nil {
		log.Stack(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	if len(taskResults) != 1 {
		errMsg := fmt.Sprintf("TaskResults size should be 1, instead of %d, project: %s, cluster: %s, job:%s, task:%d",
			len(taskResults), project, cluster, job, taskId)
		log.Errorf(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(errMsg))
		return
	}
	if taskResults[0].Err != nil {
		errMsg := fmt.Sprintf("Task failed %v", taskResults[0].Err)
		log.Errorf(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(errMsg))
		return
	}

	successStatus := map[string]string{
		"bootstrap":      StatusRunning,
		"start":          StatusRunning,
		"stop":           StatusStopped,
		"restart":        StatusRunning,
		"rolling_update": StatusRunning,
		"cleanup":        StatusNotBootstrap,
	}

	// Refresh the status if action succeed.
	for i := 0; i < len(d.clusters); i++ {
		if d.clusters[i].ClusterName == cluster {
			if jobPtr, ok := d.clusters[i].Jobs[job]; ok {
				if host, ok := jobPtr.GetHost(taskId); ok {
					host.Attributes["status"] = successStatus[action]
				}
			}
		}
	}
	w.Write([]byte("success"))
}

func (s *Dashboard) refreshCache() error {
	var err error
	s.clusters, err = s.hukerJob.List()
	if err != nil {
		return err
	}
	for i := 0; i < len(s.clusters); i++ {
		for _, job := range s.clusters[i].Jobs {
			for _, host := range job.Hosts {
				sup := huker.NewSupervisorCli(host.ToHttpAddress())
				prog, err := sup.GetTask(s.clusters[i].ClusterName, job.JobName, host.TaskId)
				status := StatusUnknown
				if err != nil {
					if !strings.Contains(err.Error(), "Task does not found") {
						log.Errorf("Get task failed: %v", err)
					} else {
						status = StatusNotBootstrap
					}
				} else {
					status = prog.Status
				}
				host.Attributes["status"] = status
			}
		}
	}
	return nil
}

// Start the dashboard server by listen the given HTTP port.
func (s *Dashboard) Start() error {
	if err := s.refreshCache(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-s.refreshTicker.C:
				if err := s.refreshCache(); err != nil {
					log.Errorf("Failed to refresh cache: %v", err)
				}
			case <-s.quit:
				s.refreshTicker.Stop()
				return
			}
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/", s.hIndex)
	r.HandleFunc("/list/{project}", s.hList)
	r.HandleFunc("/detail/{project}/{cluster}", s.hDetail)
	r.HandleFunc("/config/{project}/{cluster}/{job}/{task_id}", s.hConfig)
	r.HandleFunc("/static/{filename}", s.hStaticFile)
	r.HandleFunc("/api/{action}/{project}/{cluster}/{job}/{task_id}", s.hWebApi)
	s.srv.Handler = r

	log.Infof("Bind and listen to 0.0.0.0:%d", s.Port)
	return s.srv.ListenAndServe()
}

// Shutdown the dashboard server.
func (s *Dashboard) Shutdown() error {
	s.quit <- 1
	return s.srv.Shutdown(context.Background())
}
