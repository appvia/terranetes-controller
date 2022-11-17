#!/usr/bin/env bash
#
# Copyright 2022 Appvia Ltd <info@appvia.io>
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
set -o errexit
set -o pipefail
${TRACE:+set -x}

# Variables
LOGFILE="/tmp/build.log"
DELAY=5
MAX_RETRIES=10
LOGS_FETCHED=false
FLAG_ENDPOINT=""
FLAG_LOGFILE="${LOGFILE}"
FLAG_DELAY=${DELAY}
FLAG_MAX_RETRIES=${MAX_RETRIES}

# Help instructions for this script.
print_usage() {
  printf "\nUSAGE: $0 [flags]

Flags:
  -e  The endpoint to poll and fetch container logs (default: none)
  -f  The logfile to output build logs retrieved from the Terranetes Controller (default: $LOGFILE)
  -d  Delay in seconds between each log retrieval attempt (default: $DELAY)
  -r  Maximum log retrieval retries to attempt (default: $MAX_RETRIES)
  -h  show this help\n"
}

# Check that all required arguments have been supplied
check_required_arguments() {
  echo "[info] Checking if required flags have been provided."
  for var in "FLAG_ENDPOINT" "FLAG_LOGFILE" "FLAG_DELAY" "FLAG_MAX_RETRIES"; do
    if [ -z "${!var}" ]; then
      echo "Error: Variable ${var} has no value."
      print_usage
      exit 1
    fi
  done
}

stream_logs() {
    i=1
    while [[ ${i} -le ${FLAG_MAX_RETRIES} ]]; do
        echo "[info] Waiting ${FLAG_DELAY} seconds for pod logs to be available (attempt ${i}/${FLAG_MAX_RETRIES}).."
        sleep ${FLAG_DELAY}

        if curl --no-buffer --silent ${FLAG_ENDPOINT} | tee ${FLAG_LOGFILE}; then
            if [[ $(grep "failed to retrieve the logs" ${FLAG_LOGFILE}) ]]; then
                echo "[info] Terranetes Controller was unable to retrieve logs, the Job Pod may not be available yet."
                i=$((i+1))
            else
                LOGS_FETCHED=true
                return
            fi
        else
            echo "[warning] Terranetes Controller not reachable, unable to fetch logs."
            i=$((i+1))
        fi
    done
}

# The main function
main() {
    check_required_arguments

    stream_logs

    if [[ ${LOGS_FETCHED} == false ]]; then
        echo "[error] Unable to retrieve Job Pod logs after ${FLAG_MAX_RETRIES} attempts."
        exit 1
    fi

    if [[ $(grep "level=error" ${FLAG_LOGFILE}) ]]; then
        echo "[error] Job failure occurred, view the Terraform logs for more info."
        exit 1
    fi

    if ! grep -qi "build.*complete" ${FLAG_LOGFILE}; then
        echo "[error] An unexpected error occurred, view the Pod logs for more info."
        exit 1
    fi
}

# Parse arguments provided to the script and error if any are unexpected
while getopts 'e:f:d:r:h' flag; do
    case "${flag}" in
        e) FLAG_ENDPOINT="${OPTARG}" ;;
        f) FLAG_LOGFILE="${OPTARG}" ;;
        d) FLAG_DELAY="${OPTARG}" ;;
        r) FLAG_MAX_RETRIES="${OPTARG}" ;;
        h) print_usage && exit 0 ;;
        *) print_usage
        exit 1 ;;
    esac
done

# Run the main function
main
