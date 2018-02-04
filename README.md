# Huker

[![Go Report Card](https://goreportcard.com/badge/github.com/openinx/huker?style=flat-square)](https://goreportcard.com/report/github.com/openinx/huker)
[![Build Status Travis](https://img.shields.io/travis/openinx/huker.svg?style=flat-square&&branch=master)](https://travis-ci.org/openinx/huker)
[![Coverage Status](https://coveralls.io/repos/github/openinx/huker/badge.svg?branch=master)](https://coveralls.io/github/openinx/huker?branch=master)

# Requirements

Golang 1.8+
Java 1.8.0_51

# Build Huker

```shell
$ go get -d github.com/openinx/huker
$ cd $GOPATH/src/github.com/openinx/huker
$ make && make test
```

# Quick Start

Let's bootstrap a pseudo-distribute Hadoop cluster under your local host, which means it'll start multiple services on one single host but listen different tcp ports.

#### Prepare

* Start huker package manager

```
$ ./bin/huker start-pkg-manager
```

Note: Huker Package Manager will take serveral minutes to sync all release packages defined in `./conf/pkg.yaml` to your localhost, and all huker agent will download packages from huker package manager.

* Start a huker agent

```
$ ./bin/huker start-agent --dir /tmp/huker --file /tmp/agent01.db
```

After all release packages have been synced to your localhost successfully , Let's start following:

#### Step.1 Bootstrap a zookeeper cluster with 3 node.

```
$ ./bin/huker bootstrap zookeeper test-zk zkServer
```

You can show your zkServer job status by:

```
$ ./bin/huker show zookeeper test-zk zkServer
```

Besides, you can find all your jobs by typing http://127.0.0.1:9001 in your browser.

#### Step.2 Bootstrap a HDFS cluster with 1 namenode and 4 datanode.

```
$ ./bin/huker bootstrap hdfs test-hdfs namenode
$ ./bin/huker bootstrap hdfs test-hdfs datanode
```

#### Step.3 Bootstrap a Yarn cluster with 1 resource manager and 1 node manager

```
$ ./bin/huker bootstrap yarn test-yarn resourcemanager
$ ./bin/huker bootstrap yarn test-yarn nodemanager
```

#### Step.4 Bootstrap a HBase cluster based on previous test-zk cluster and test-hdfs cluster.

```
$ ./bin/huker bootstrap hbase test-hbase master
$ ./bin/huker bootstrap hbase test-hbase regionserver
```
