#!/usr/bin/env bash

#
# This hack of a script can be deleted once the following issue has been fixed:
#
#   https://github.com/kubernetes-sigs/controller-tools/issues/476
#
# Unfortunately, `yq` does not seem to support writing multiple files at once,
# so it is necessary to loop over all the files and invoke `yq` for each one
# individually. Compiling `yq` once and then invoking the binary is a lot
# faster than using `go run github.com/mikefarah/yq/v3` for each file.
#

set -euo pipefail
${TRACE:+set -x}

BIN_DIR=./bin
YQ="$BIN_DIR/yq"

cleanup() {
  rm -f "$YQ"
}
trap cleanup EXIT INT TERM

build_yq() {
  mkdir -p "$BIN_DIR"
  go build -o "$YQ" github.com/mikefarah/yq/v3
}

add_preserveUnknownFields() {
  for f in ./charts/terranetes-controller/crds/*.yaml; do
    "$YQ" write --inplace "$f" spec.preserveUnknownFields false
  done
}

build_yq
add_preserveUnknownFields
