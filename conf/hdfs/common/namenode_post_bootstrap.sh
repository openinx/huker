#!/usr/bin/env bash

# The following environment variables can be used in your hook script:
#   SUPERVISOR_ROOT_DIR
#   PROGRAM_BIN
#   PROGRAM_ARGS
#   PROGRAM_DIR
#   PROGRAM_NAME
#   PROGRAM_JOB_NAME
#   PROGRAM_TASK_ID

HOOK_SHELL="$PROGRAM_BIN $PROGRAM_ARGS -format"
echo $HOOK_SHELL
eval $HOOK_SHELL