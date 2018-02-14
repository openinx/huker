#!/usr/bin/env bash

PID=`ps -ef | grep huker-pkg-manager | grep start-pkg-manager  | grep -v grep | grep -v ssh | awk '{print $2}'`
if [ "$PID" != "" ]; then
      echo "Kill huker-pkg-manager process." $PID
      kill -9 $PID
fi

nohup {{ huker_install_dir }}/huker \
      --log-file {{ huker_install_dir }}/huker-pkg-manager.log \
      start-pkg-manager \
      --dir {{ huker_root_dir }}/lib \
      --port {{ huker_pkg_manager_port }} \
      --conf {{ huker_install_dir }}/pkg.yaml >/dev/null 2>&1 &
