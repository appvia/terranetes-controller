#!/usr/bin/env bash
#
# Copyright 2021 Appvia Ltd <info@appvia.io>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Retry a command $1 times until it succeeds. If a second command is given, it will be run on the output of the first command.
retry() {
  if [ $# -gt 3 ]; then
    echo "Invalid number of arguments for retry: \"$*\" ($#)"
    exit 1
  fi

  local attempts=$1
  shift
  local cmd=$1
  shift

  local subcmd
  if [ $# -gt 0 ]; then
    subcmd=$1
    shift
  fi

  local delay=5
  local i
  local result

  for ((i = 1; i <= attempts; i++)); do
    run bash -c "$cmd"
    result="${output}"

    if [[ -z ${subcmd}   ]]; then
      if [[ ${status} -eq 0   ]]; then
        echo "${result}"
        return 0
      fi
    else
      run bash -c "${subcmd}" < <(echo -n "${result}")
      if [[ ${status} -eq 0   ]]; then
        echo "${output}"
        return 0
      fi
    fi

    if [[ $i -lt $attempts ]]; then
      sleep $delay
    fi
  done

  printf "Error: command \"%s\" (subcommand: \"%s\") failed %d times\nStatus: %s\nOutput: %s\n" "${cmd}" "${subcmd}" "${attempts}" "${status}" "${result}" | sed 's/WAYFINDER_TOKEN=.* //' >&2
  false
}

runit() {
  retry 5 "$@"
}
