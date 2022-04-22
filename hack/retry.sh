#!/usr/bin/env bash
#
# Used to retry a command x amount of times
#

set -euo pipefail

i=0
until [[ "$i" -ge 3 ]]; do
  "$@" && exit 0
  i=$((i++))
  sleep 1
done

echo "failed to execute command: $@ successfully"
exit 1
