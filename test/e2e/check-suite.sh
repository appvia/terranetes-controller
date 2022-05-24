#!/bin/bash
#
# Copyright (C) 2022  Appvia Ltd <info@appvia.io>
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; either version 2
# of the License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

APP_NAMESPACE="apps"
BATS_OPTIONS=${BATS_OPTIONS:-""}
BUCKET=${BUCKET:-"terraform-controller-ci-bucket"}
CLOUD="aws"
UNITS="test/e2e/integration"

usage() {
  cat <<EOF
Usage: $0 [options]
--cloud <NAME>         Cloud provider name to run against (aws, azure, google, defaults: aws)
--help                 Display this help message
EOF
  if [[ -n "${@}" ]]; then
    echo "Error: ${1}"
    exit 1
  fi

  exit 0
}

run_bats() {
  echo "Running units: ${@}"
  APP_NAMESPACE=${APP_NAMESPACE} \
  BUCKET=${BUCKET} \
  CLOUD=${CLOUD} \
  RESOURCE_NAME=bucket-${CLOUD} \
  NAMESPACE="terraform-system" \
  bats ${BATS_OPTIONS} ${@} || exit 1
}

# run-checks runs a collection checks
run_checks() {
  local FILES=(
    "${UNITS}/setup.bats"
    "${UNITS}/${CLOUD}/provider.bats"
    "${UNITS}/${CLOUD}/plan.bats"
    "${UNITS}/plan.bats"
    "${UNITS}/apply.bats"
    "${UNITS}/${CLOUD}/confirm.bats"
    "${UNITS}/destroy.bats"
    "${UNITS}/${CLOUD}/destroy.bats"
  )
  echo "Running suite on: ${CLOUD^^}"
  for filename in "${FILES[@]}"; do
    if [[ -f "${filename}" ]]; then
      run_bats ${filename} || exit 1
    fi
  done
}

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --cloud)
      CLOUD="${2}"
      shift 2
      ;;
    --help)
      usage
      ;;
    *)
      usage "Unknown argument: ${1}"
      ;;
  esac
done

[[ ${CLOUD} == "aws" ]] || [[ ${CLOUD} == "azure" ]] || [[ ${CLOUD} == "google" ]] || usage "Unknown cloud: ${CLOUD}"

run_checks
