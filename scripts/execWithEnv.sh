#!/bin/bash
#
# execWithEnv.sh
# Run any executable with the variables from a dotenv file loaded into the
# environment. Handy for launching a compiled bin/goeland-server against a
# specific .env without polluting your shell.
#
# Usage:
#   ./scripts/execWithEnv.sh <path-to-executable> [env-file]
# Examples:
#   ./scripts/execWithEnv.sh bin/goeland-server            # uses .env
#   ./scripts/execWithEnv.sh bin/goeland-server .env_dev   # uses .env_dev

echo "## $0 received NUM ARGS : " $#
ENV_FILENAME='.env'
if [ $# -eq 1 ]; then
  BIN_FILENAME=${1}
elif [ $# -eq 2 ]; then
  BIN_FILENAME=${1}
  ENV_FILENAME=${2:-'.env'}
else
  echo "## 💥💥 expecting first argument to be an executable path and an optional second argument for the .env file name"
  exit 1
fi

echo "## will try to run : ${BIN_FILENAME} with env variables in ${ENV_FILENAME} ..."
if [ -r "$ENV_FILENAME" ]; then
  if [ -x "$BIN_FILENAME" ]; then
    echo "## will execute $BIN_FILENAME"
    # Load the dotenv file: drop comments/blank lines, escape single quotes and
    # single-quote every value so spaces and special characters survive. `set -a`
    # exports everything sourced in between.
    set -a
    source <(sed -e '/^#/d;/^\s*$/d' -e "s/'/'\\\''/g" -e "s/=\(.*\)/='\1'/g" "$ENV_FILENAME")
    ${BIN_FILENAME}
    set +a
  else
    echo "## 💥💥 expecting first argument to be an executable path"
    exit 1
  fi
else
  echo "## 💥💥 env path argument : ${ENV_FILENAME} was not found"
  exit 1
fi
