#!/bin/bash

set -e
${TRACE:+set -x}

if [ $# == 0 ]; then
  echo "usage: $0 [path ...]"
  exit 1
fi

default_params="-w"

dry_run=false
for arg in "$@"; do
  if [[ "$arg" == "-l" ]]; then
    dry_run=true
    default_params=""
  fi
done

remove_lines_file() {
  if LANG=C sed --help 2>&1 | grep -q GNU; then
    sed -i '
      /^import/,/)/ {
        /^$/ d
      }
    ' "${1}"
  else
    sed -i .orig '
      /^import/,/)/ {
        /^$/ d
      }
    ' "${1}"
    rm -f -- "${1}.orig"

  fi
}

remove_lines() {
  if [[ -f "${1}" ]] && [[ "${1}" == *.go ]]; then
    remove_lines_file "${1}"
  elif [[ -d "${1}" ]]; then
    for f in ${1}/*; do
      remove_lines "${f}"
    done
  fi
}

if [[ "${dry_run}" == "false" ]]; then
  for arg in "$@"; do
    remove_lines "$arg"
  done
fi

go run golang.org/x/tools/cmd/goimports -local github.com/appvia/wayfinder ${default_params} "$@"
