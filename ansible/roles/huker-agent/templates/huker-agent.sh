#!/usr/bin/env bash

PID=`ps -ef | grep huker-agent | grep start-agent  | grep -v grep | grep -v ssh | awk '{print $2}'`
if [ "$PID" != "" ]; then
      echo "Kill huker-agent process." $PID
      kill -9 $PID
fi

nohup {{ huker_install_dir }}/huker \
      --log-file {{ huker_install_dir }}/huker-agent.log \
      start-agent \
      --dir {{ huker_root_dir }} \
      --port {{ huker_agent_port }} \
      --file {{ huker_install_dir }}/huker-agent.db >/dev/null 2>&1 &
