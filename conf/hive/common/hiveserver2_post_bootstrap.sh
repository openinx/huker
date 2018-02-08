#!/usr/bin/env bash

# The following environment variables can be used in your hook script:
#   SUPERVISOR_ROOT_DIR
#   PROGRAM_BIN
#   PROGRAM_ARGS
#   PROGRAM_DIR
#   PROGRAM_NAME
#   PROGRAM_JOB_NAME
#   PROGRAM_TASK_ID

SCHEMA_TOOL_ARGS=`echo $PROGRAM_ARGS | sed -e 's/org.apache.hive.service.server.HiveServer2/org.apache.hive.beeline.HiveSchemaTool/g'`
HOOK_SHELL="$PROGRAM_BIN $SCHEMA_TOOL_ARGS -dbType derby -initSchema"
echo $HOOK_SHELL

export HIVE_HOME=$PROGRAM_DIR/pkg
eval $HOOK_SHELL
