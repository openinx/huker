package dashboard

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	huker "github.com/openinx/huker/pkg/core"
	"github.com/openinx/huker/pkg/supervisor"
	"github.com/openinx/huker/pkg/utils"
	"github.com/qiniu/log"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

type Dashboard struct {
	Port          int
	srv           *http.Server
	hukerJob      huker.HukerJob
	refreshTicker *time.Ticker
	quit          chan int
	clusters      []*huker.Cluster
}

func NewRawDashboard(port int, configRootDir, pkgServerAddress string) (*Dashboard, error) {
	hukerJob, err := huker.NewConfigFileHukerJob(configRootDir, pkgServerAddress)
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

// Create a new supervisor agent.
func NewDashboard(port int) (*Dashboard, error) {
	configRootDir := utils.ReadEnvStrValue(huker.HUKER_CONF_DIR, path.Join(utils.GetHukerDir(), huker.HUKER_CONF_DIR_DEFAULT))
	pkgServerAddress := utils.ReadEnvStrValue(huker.HUKER_PKG_HTTP_SERVER, huker.HUKER_PKG_HTTP_SERVER_DEFAULT)
	return NewRawDashboard(port, configRootDir, pkgServerAddress)
}

type HandleFunc func(w http.ResponseWriter, r *http.Request) (string, error)

func handleResponse(w http.ResponseWriter, r *http.Request, handleFunc HandleFunc) {
	body, err := handleFunc(w, r)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}
}

func (d *Dashboard) hIndex(w http.ResponseWriter, r *http.Request) {
	handleResponse(w, r, func(w http.ResponseWriter, r *http.Request) (string, error) {
		return RenderTemplate("site/overview.html", "site/base.html", map[string]interface{}{}, nil)
	})
}

func (d *Dashboard) hList(w http.ResponseWriter, r *http.Request) {
	handleResponse(w, r, func(w http.ResponseWriter, r *http.Request) (string, error) {
		project := mux.Vars(r)["project"]
		clusters, err := d.hukerJob.List()
		if err != nil {
			return "", err
		}
		var projectClusters []*huker.Cluster
		for i := 0; i < len(clusters); i++ {
			if clusters[i].Project == project {
				projectClusters = append(projectClusters, clusters[i])
			}
		}

		return RenderTemplate("site/list-cluster.html", "site/base.html", map[string]interface{}{
			"project":  project,
			"clusters": projectClusters}, nil)
	})
}

func (d *Dashboard) hDetail(w http.ResponseWriter, r *http.Request) {
	handleResponse(w, r, func(w http.ResponseWriter, r *http.Request) (string, error) {
		project := mux.Vars(r)["project"]
		clusterName := mux.Vars(r)["cluster"]
		var cluster *huker.Cluster
		for i := 0; i < len(d.clusters); i++ {
			if d.clusters[i].Project == project && d.clusters[i].ClusterName == clusterName {
				cluster = d.clusters[i]
			}
		}
		if cluster == nil {
			return "", fmt.Errorf("Cluster not found. project:%s, cluster:%s", project, clusterName)
		}

		return RenderTemplate("site/detail.html", "site/base.html", map[string]interface{}{
			"cluster": cluster,
		}, template.FuncMap{
			"inc": func(i int) int {
				return i + 1
			},
		})
	})
}

func (d *Dashboard) hConfig(w http.ResponseWriter, r *http.Request) {
	handleResponse(w, r, func(w http.ResponseWriter, r *http.Request) (string, error) {
		project := mux.Vars(r)["project"]
		clusterName := mux.Vars(r)["cluster"]
		jobName := mux.Vars(r)["job"]
		taskId, err := strconv.Atoi(mux.Vars(r)["task_id"])
		if err != nil {
			return "", err
		}

		var cluster *huker.Cluster
		for i := 0; i < len(d.clusters); i++ {
			if d.clusters[i].Project == project && d.clusters[i].ClusterName == clusterName {
				cluster = d.clusters[i]
			}
		}
		if cluster == nil {
			return "", fmt.Errorf("Cluster not found. project:%s, cluster:%s", project, clusterName)
		}
		job := cluster.Jobs[jobName]
		if job == nil {
			return "", fmt.Errorf("Job not found. project:%s, cluster:%s, job:%s", project, clusterName, jobName)
		}
		configMap, err := cluster.RenderConfigFiles(job, taskId, false)
		if err != nil {
			return "", fmt.Errorf("Render configuration files failed, project:%s, cluster:%s, job:%s,task:%d, err: %v",
				project, clusterName, jobName, taskId, err)
		}

		isFirst := 1
		return RenderTemplate("site/config.html", "site/base.html", map[string]interface{}{
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
	})
}

func (d *Dashboard) hStaticFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "site/static/"+mux.Vars(r)["filename"])
}

func (d *Dashboard) hWebApi(w http.ResponseWriter, r *http.Request) {
	handleResponse(w, r, func(w http.ResponseWriter, r *http.Request) (string, error) {
		action := mux.Vars(r)["action"]
		project := mux.Vars(r)["project"]
		cluster := mux.Vars(r)["cluster"]
		job := mux.Vars(r)["job"]
		taskId, err := strconv.Atoi(mux.Vars(r)["task_id"])
		if err != nil {
			return "", fmt.Errorf("task_id should be a integer, instead of %s", mux.Vars(r)["task_id"])
		}

		var taskResults []huker.TaskResult
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
			return "", fmt.Errorf("Unsupported action: " + action)
		}
		if err != nil {
			log.Stack(err)
			return "", err
		}
		if len(taskResults) != 1 {
			return "", fmt.Errorf("TaskResults size should be 1, instead of %d, project: %s, cluster: %s, job:%s, task:%d",
				len(taskResults), project, cluster, job, taskId)
		}
		if taskResults[0].Err != nil {
			return "", taskResults[0].Err
		}

		successStatus := map[string]string{
			"bootstrap":      supervisor.StatusRunning,
			"start":          supervisor.StatusRunning,
			"stop":           supervisor.StatusStopped,
			"restart":        supervisor.StatusRunning,
			"rolling_update": supervisor.StatusRunning,
			"cleanup":        supervisor.StatusNotBootstrap,
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
		return "", nil
	})
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
				sup := supervisor.NewSupervisorCli(host.ToHttpAddress())
				prog, err := sup.GetTask(s.clusters[i].ClusterName, job.JobName, host.TaskId)
				status := supervisor.StatusUnknown
				if err != nil {
					if !strings.Contains(err.Error(), "Task does not found") {
						log.Errorf("Get task failed: %v", err)
					} else {
						status = supervisor.StatusNotBootstrap
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
