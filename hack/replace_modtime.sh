#!/usr/bin/env bash

set -e
${TRACE:+set -x}

endswith() { case $1 in *"$2") true ;; *) false ;; esac; }

if [ $# = 0 ]; then
  echo "usage: $0 [path ...]"
  exit 1
fi

replace_modtime_file() {
  sed_cmd="s/modTime:([ \t]*)time.Unix[(][^)]*[)]/modTime:\1time.Unix(0, 0)/g"
  if LANG=C sed --help 2>&1 | grep -q GNU; then
    sed -E -i "$sed_cmd" "${1}"
  else
    sed -E -i .orig "$sed_cmd" "${1}"
    rm "${1}.orig"
  fi
}

replace_modtime() {
  if [ -f "${1}" ]; then
    replace_modtime_file "${1}"
  elif [ -d "${1}" ]; then
    for f in ${1}/*; do
      replace_modtime "${f}"
    done
  fi
}

for arg in "$@"; do
  replace_modtime "$arg"
done
