package main

import (
	"fmt"
	"github.com/openinx/huker"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
	"os"
	"path/filepath"
)

func logConsole(action string, job string, results []huker.TaskResult) {
	if results != nil {
		for i := range results {
			if results[i].Err != nil {
				log.Errorf("%s job %s at %s -> Failed, %v", action, job, results[i].Host.ToKey(), results[i].Err)
			} else if results[i].Prog != nil {
				log.Infof("%s job %s at %s -> %s", action, job, results[i].Host.ToKey(), results[i].Prog.Status)
			} else {
				log.Infof("%s job %s at %s -> Success", action, job, results[i].Host.ToKey())
			}
		}
	} else {
		log.Warnf("%s job %s -> No task found.", action, job)
	}
}

func handleShellAction(action string, c *cli.Context) error {
	h, err := huker.NewDefaultHukerJob()
	if err != nil {
		return err
	}

	var results []huker.TaskResult
	switch action {
	case "Install":
		results, err = h.Install(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Bootstrap":
		results, err = h.Bootstrap(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Start":
		results, err = h.Start(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Stop":
		results, err = h.Stop(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Show":
		results, err = h.Show(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Restart":
		results, err = h.Restart(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "RollingUpdate":
		results, err = h.RollingUpdate(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	case "Cleanup":
		results, err = h.Cleanup(c.String("project"), c.String("cluster"), c.String("job"), c.Int("task"))
	default:
		return fmt.Errorf("Unsupported action: %s", action)
	}
	if err != nil {
		return err
	} else {
		logConsole(action, c.String("job"), results)
		return nil
	}
}

func main() {

	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name:  "bootstrap",
			Usage: "Bootstrap a cluster of specific project, or jobs, or tasks",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name of the project, for hbase, the job will be master, regionserver, canary etc.",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job, type: integer",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Bootstrap", c)
			},
		},
		{
			Name:  "show",
			Usage: "Show jobs of a given service",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Show", c)
			},
		},
		{
			Name:  "start",
			Usage: "Start a job",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Start", c)
			},
		},
		{
			Name:  "cleanup",
			Usage: "Cleanup a job",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Cleanup", c)
			},
		},
		{
			Name:  "rolling_update",
			Usage: "Rolling update a job",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("RollingUpdate", c)
			},
		},
		{
			Name:  "restart",
			Usage: "Restart a job",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Restart", c)
			},
		},
		{
			Name:  "stop",
			Usage: "Stop a job",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "project",
					Usage: "project name, such as hdfs, yarn, zookeeper, hbase, etc",
				},
				cli.StringFlag{
					Name:  "cluster",
					Usage: "cluster name",
				},
				cli.StringFlag{
					Name:  "job",
					Usage: "job name",
				},
				cli.IntFlag{
					Name:   "task",
					Value:  -1,
					Hidden: true,
					Usage:  "task id of given service and job",
				},
			},
			Action: func(c *cli.Context) error {
				return handleShellAction("Stop", c)
			},
		},
		{
			Name:  "start-agent",
			Usage: "Start Huker Agent",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "dir, d",
					Value: ".",
					Usage: "Root directory of huker agent.",
				},
				cli.IntFlag{
					Name:  "port, p",
					Value: 9001,
					Usage: "Port to listen for huker agent.",
				},
				cli.StringFlag{
					Name:  "file, f",
					Value: "./supervisor.db",
					Usage: "file to store process meta.",
				},
			},
			Action: func(c *cli.Context) error {
				dir := c.String("dir")
				port := c.Int("port")
				file := c.String("file")
				if absDir, err := filepath.Abs(dir); err != nil {
					return err
				} else if s, err := huker.NewSupervisor(absDir, port, file); err != nil {
					return err
				} else {
					return s.Start()
				}
			},
		},
		{
			Name:  "start-pkg-manager",
			Usage: "Start Huker Package Manager",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "port, p",
					Value: 4000,
					Usage: "Port to listen for huker package manager",
				},
				cli.StringFlag{
					Name:  "dir, d",
					Value: "./lib",
					Usage: "Libaray directory of huker package manager for downloading package",
				},
				cli.StringFlag{
					Name:  "conf, c",
					Value: "./conf/pkg.yaml",
					Usage: "Configuration file of huker package manager",
				},
			},
			Action: func(c *cli.Context) error {
				port := c.Int("port")
				dir := c.String("dir")
				conf := c.String("conf")
				if absDir, err := filepath.Abs(dir); err != nil {
					return err
				} else if p, err := huker.NewPackageServer(port, absDir, conf); err != nil {
					return err
				} else {
					return p.Start()
				}
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
	}
}
