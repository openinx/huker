package main

import (
	"fmt"
	"github.com/openinx/huker"
	"github.com/qiniu/log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func handleClusterAction(action string, project, cluster, job string, taskId int, extraArgs []string) error {
	h, err := huker.NewDefaultHukerJob()
	if err != nil {
		return err
	}

	var results []huker.TaskResult
	switch action {
	case "install":
		results, err = h.Install(project, cluster, job, taskId)
	case "bootstrap":
		results, err = h.Bootstrap(project, cluster, job, taskId)
	case "start":
		results, err = h.Start(project, cluster, job, taskId)
	case "stop":
		results, err = h.Stop(project, cluster, job, taskId)
	case "show":
		results, err = h.Show(project, cluster, job, taskId)
	case "restart":
		results, err = h.Restart(project, cluster, job, taskId)
	case "rolling_update":
		results, err = h.RollingUpdate(project, cluster, job, taskId)
	case "cleanup":
		results, err = h.Cleanup(project, cluster, job, taskId)
	case "shell":
		// TODO implement this parts.
		return fmt.Errorf("project: %s, cluster: %s, job: %s, taskId: %d, extra_args: %v", project, cluster, job, taskId, extraArgs)
	default:
		return fmt.Errorf("Unsupported command: %s", action)
	}
	if err != nil {
		return err
	} else {
		logConsole(action, job, results)
		return nil
	}
}

func printUsageAndExit() {
	fmt.Println("Usage: huker [<options> <command> <args>]")
	fmt.Println("Options: ")
	fmt.Println("  --log-level INFO|DEBUG|WARN|ERROR   Log level when execute the command")
	fmt.Println("  --log-file  FILE                    File to write the log.")
	fmt.Println("Commands: ")
	fmt.Println("Some commands take arguments, Pass no args for usage.")
	fmt.Println("  shell               Run the shell for specified job")
	fmt.Println("  bootstrap           Bootstrap the job to install packages and start the job")
	fmt.Println("  show                Show the job status")
	fmt.Println("  cleanup             Cleanup the packages")
	fmt.Println("  rolling_update      Rolling update the configuration files and packages for job")
	fmt.Println("  restart             Restart the job")
	fmt.Println("  start               Start the job")
	fmt.Println("  stop                Stop the job")
	fmt.Println("  start-agent         Start the supervisor agent")
	fmt.Println("    --dir,-d          Root directory of huker agent (default: .)")
	fmt.Println("    --port,-p         Port to listen for huker agent (default: 9001)")
	fmt.Println("    --file,-f         File to store process meta. (default: ./supervisor.db)")
	fmt.Println("  start-pkg-manager   Start the package manager http server")
	fmt.Println("    --dir,-d          Libaray directory of huker package manager for downloading package (default: ./lib)")
	fmt.Println("    --port,-p         Port of http server to listen (default: 4000)")
	fmt.Println("    --conf,-c         Configuration file of huker package manager (default: ./conf/pkg.yaml)")
	os.Exit(1)
}

func handleAction(command string, args []string) {
	if len(args) < 3 {
		fmt.Printf("Command %s: not enough arguments\n", command)
		fmt.Printf("Usage: %s <project> <cluster> <job> [<task_id>]\n", command)
		os.Exit(1)
	}
	project, cluster, job, taskId := args[0], args[1], args[2], -1
	index := 3
	if command != "shell" && index < len(args) {
		var err error
		taskId, err = strconv.Atoi(args[index])
		if err != nil {
			fmt.Printf("<task_id> shoud be int, instead of %s\n", args[index])
			os.Exit(1)
		}
		index++
	}
	if err := handleClusterAction(command, project, cluster, job, taskId, args[index:]); err != nil {
		log.Error(err)
	}
}

func main() {

	if len(os.Args) < 2 {
		printUsageAndExit()
	}

	index := 1
	for ; index+1 < len(os.Args) && strings.HasPrefix(os.Args[index], "-"); index += 2 {
		if os.Args[index] == "--log-level" {
			switch os.Args[index+1] {
			case "INFO":
				log.SetOutputLevel(log.Linfo)
			case "DEBUG":
				log.SetOutputLevel(log.Ldebug)
			case "WARN":
				log.SetOutputLevel(log.Lwarn)
			case "ERROR":
				log.SetOutputLevel(log.Lerror)
			default:
				fmt.Printf("Invalid log level: %s", os.Args[index+1])
				printUsageAndExit()
			}
		} else if os.Args[index] == "--log-file" {
			f, err := os.OpenFile(os.Args[index+1], os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				log.Error(err)
				os.Exit(1)
			}
			log.SetOutput(f)
		}
	}

	if index >= len(os.Args) {
		fmt.Println("Command not found.")
		printUsageAndExit()
	}

	command := os.Args[index]
	index++
	for _, cmd := range []string{"shell", "bootstrap", "show", "cleanup", "rolling_update", "restart", "stop", "start"} {
		if cmd == command {
			handleAction(command, os.Args[index:])
			return
		}
	}

	if command == "start-agent" {
		dir, _ := filepath.Abs(".")
		port := 9001
		file, _ := filepath.Abs("./supervisor.db")
		for ; index+1 < len(os.Args); index += 2 {
			if os.Args[index] == "-d" || os.Args[index] == "--dir" {
				dir = os.Args[index+1]
			} else if os.Args[index] == "-f" || os.Args[index] == "--file" {
				file = os.Args[index+1]
			} else if os.Args[index] == "-p" || os.Args[index] == "--port" {
				newPort, err := strconv.Atoi(os.Args[index+1])
				if err != nil {
					fmt.Printf("port shoud be int, not %s\n", os.Args[index+1])
					printUsageAndExit()
				}
				port = newPort
			} else {
				fmt.Printf("Unexpected arguments: %v\n", os.Args[index:])
				printUsageAndExit()
			}
		}
		if index < len(os.Args) {
			fmt.Printf("Unexpected arguments: %v\n", os.Args[index:])
			printUsageAndExit()
		}
		if supervisor, err := huker.NewSupervisor(dir, port, file); err != nil {
			log.Stack(err)
			return
		} else if err := supervisor.Start(); err != nil {
			log.Stack(err)
			return
		}
	} else if command == "start-pkg-manager" {
		pkgRoot, _ := filepath.Abs("./lib")
		port := 4000
		pkgConf, _ := filepath.Abs("./conf/pkg.yaml")
		for ; index+1 < len(os.Args); index += 2 {
			if os.Args[index] == "-d" || os.Args[index] == "--dir" {
				pkgRoot = os.Args[index+1]
			} else if os.Args[index] == "-c" || os.Args[index] == "--conf" {
				pkgConf = os.Args[index+1]
			} else if os.Args[index] == "-p" || os.Args[index] == "--port" {
				newPort, err := strconv.Atoi(os.Args[index+1])
				if err != nil {
					fmt.Printf("port shoud be int, not %s\n", os.Args[index+1])
					printUsageAndExit()
				}
				port = newPort
			} else {
				fmt.Printf("Unexpected arguments: %v\n", os.Args[index:])
				printUsageAndExit()
			}
		}
		if index < len(os.Args) {
			fmt.Printf("Unexpected arguments: %v\n", os.Args[index:])
			printUsageAndExit()
		}
		if pkgSrv, err := huker.NewPackageServer(port, pkgRoot, pkgConf); err != nil {
			log.Stack(err)
			return
		} else if err := pkgSrv.Start(); err != nil {
			log.Stack(err)
			return
		}
	} else {
		fmt.Printf("No help topic for '%s'\n", command)
		printUsageAndExit()
	}
}
