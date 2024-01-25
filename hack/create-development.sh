#!/usr/bin/env bash
#
# Copyright (C) 2023  Appvia Ltd <info@appvia.io>
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

set -e

HELM_VALUES_FILE="dev/my_values.yaml"
CLOUD_CREDENTIALS_FILE="dev/dev.creds"

usage() {
  cat << EOF >&2
Usage: $0 [OPTIONS] [NAME]
  --values PATH   Is the path to the values file for helm deployment
  --creds PATH    Is the path to the cloud credentials file (environment variables i.e. AWS_ACCESS_KEY_ID)
EOF

  if [[ ${*} -gt 0  ]]; then
    echo -e "\n${*}" >&2
    exit 1
  fi

  exit 0
}

create_environment() {
  if ! kind get clusters | grep -q kind; then
    echo -e "Provision the Kubernetes cluster via kind"
    if ! kind create cluster; then
      echo -e "Failed to create the Kubernetes cluster via kind"
      exit 1
    fi

    echo -e "Provision the Images in Kind Cluster"
    if ! make controller-kind; then
      echo -e "Failed to provision the Images in Kind Cluster"
      exit 1
    fi
  fi

  echo -e "Provision the cloud credentials secret in controller namespace"
  source "${PWD}/${CLOUD_CREDENTIALS_FILE}" || {
    echo -e "Failed to source the cloud credentials file"
    exit 1
  }
  if ! kubectl -n terraform-system get secret aws > /dev/null 2>&1; then
    make aws-credentials || {
      echo -e "Failed to create the aws cloud credentials secret"
      exit 1
    }
  fi

  echo -e "Provision the terranetes controller helm release"
  if ! helm upgrade --install terranetes-controller ./charts/terranetes-controller --namespace terraform-system --values ${HELM_VALUES_FILE}; then
    echo -e "Failed to provision the terranetes controller helm release"
    exit 1
  fi

  echo -e "Provision the cloud provider"
  if ! kubectl apply -f examples/provider.yaml; then
    echo -e "Failed to provision the aws cloud provider"
    exit 1
  fi
}

while [[ ${#} -gt 0   ]]; do
  case "${1}" in
    --values)
      HELM_VALUES_FILE="${2}"
      shift 2
      ;;
    --creds)
      CLOUD_CREDENTIALS_FILE="${2}"
      shift 2
      ;;
    --help)
      usage
      ;;
    *)
      usage "Unknown option: ${1}"
      ;;
  esac
done

[[ ${CLOUD_CREDENTIALS_FILE} == ""   ]] && usage "Missing cloud credentials file"
[[ ${HELM_VALUES_FILE} == ""   ]] && usage "Missing helm values file"
[[ -e ${CLOUD_CREDENTIALS_FILE}   ]] || usage "Cloud credentials file does not exist"
[[ -e ${HELM_VALUES_FILE}   ]] || usage "Helm values file does not exist"

create_environment
