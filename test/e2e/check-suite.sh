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
CLOUD=""
UNITS="test/e2e/integration"
VERSION="ci"

usage() {
  cat <<EOF
Usage: $0 [options]
--cloud <NAME>         Cloud provider name to run against (aws, azure, google, defaults: aws)
--version <TAG>        Version of the Terraform Controller to test against (defaults: ${VERSION})
--help                 Display this help message
EOF
  if [[ -n "${@}" ]]; then
    echo "Error: ${1}"
    exit 1
  fi

  exit 0
}

run_bats() {
  echo -e "Running units: ${@}\n"
  APP_NAMESPACE=${APP_NAMESPACE} \
  BUCKET=${BUCKET} \
  CLOUD=${CLOUD} \
  RESOURCE_NAME=bucket-${CLOUD:-"test"} \
  NAMESPACE="terraform-system" \
  VERSION=${VERSION} \
  bats ${BATS_OPTIONS} ${@} || exit 1
}

# run-checks runs a collection checks
run_checks() {
  local CLOUD_FILES=(
    "${UNITS}/cloud/${CLOUD}/provider.bats"
    "${UNITS}/cloud/${CLOUD}/plan.bats"
    "${UNITS}/plan.bats"
    "${UNITS}/costs.bats"
    "${UNITS}/apply.bats"
    "${UNITS}/cloud/${CLOUD}/confirm.bats"
    "${UNITS}/drift.bats"
    "${UNITS}/destroy.bats"
    "${UNITS}/cloud/${CLOUD}/destroy.bats"
    "${UNITS}/cloud/${CLOUD}/costs.bats"
    "${UNITS}/private.bats"
  )
  local CONSTRAINTS_FILES=(
    "${UNITS}/constraints/setup.bats"
    "${UNITS}/constraints/modules.bats"
  )

  # Run in the installation
  run_bats "${UNITS}/setup.bats"
  if [[ -n "${CLOUD}" ]]; then
    echo -e "Running suite on: ${CLOUD^^}\n"
    for x in "${CLOUD_FILES[@]}"; do
      if [[ -f "${x}" ]]; then
        run_bats ${x} || exit 1
      fi
    done
  fi
  for x in "${CONSTRAINTS_FILES[@]}"; do
    if [[ -f "${x}" ]]; then
      run_bats ${x} || exit 1
    fi
  done
}

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --cloud)
      CLOUD="${2}"
      shift 2
      ;;
    --version)
      VERSION="${2}"
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

[[ ${CLOUD} == "aws" ]] || [[ ${CLOUD} == "azure" ]] || [[ ${CLOUD} == "google" ]] || [[ ${CLOUD} == "" ]] || usage "Unknown cloud: ${CLOUD}"

run_checks
