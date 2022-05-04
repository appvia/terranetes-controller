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

CHECKOV_POLICY_URL=""
COST_REPORT_FILE="/tmp/costs.json"
ENABLE_CHECKOV="${ENABLE_CHECKOV:-false}"
ENABLE_INFRACOST=${ENABLE_INFRACOST:-"false"}
INFRACOST=${INFRACOST:-"/usr/bin/infracost"}
INFRACOST_API_KEY=${INFRACOST_API_KEY:-""}
INFRACOST_API_URL=${INFRACOST_API_URL:-""}
TERRAFORM=${TERRAFORM:-"/usr/bin/terraform"}
TERRAFORM_PLAN="/tmp/plan.out"
TERRAFORM_PLAN_JSON="/tmp/plan.json"
TERRAFORM_VARIABLES=""

export NC='\e[0m'
export GREEN='\e[0;32m'
export YELLOW='\e[0;33m'
export RED='\e[0;31m'

# log will echo the message and reset colour code
log() { (printf 2>/dev/null "%b\n" "$*${NC}"); }
# announce will log a given message (used for standard info logging)
announce() { log "${GREEN}[info] $*"; }
# failed is to notify of configuration failures (e.g. missing files / environment variables)
failed() { log "${YELLOW}[fail] $*"; }
# error is used when unexpected errors occur (e.g. unable to communicate with API)
error() { log "${RED}[error] $*"; }
# fatal is used when an unrecoverable error occurs and execution should be stopped
fatal() {
  error "$*"
  exit 1
}

usage() {
  cat <<EOF
Use: ${0} [OPTIONS]
-apply          Applys the terraform plan
-checkov-url    The git url which needs to be cloned, containing the policies
-cost-api       The infracost api url (defaults to officially hosted
-cost-token     The infracost api token
-destroy        Destroy any currently deployed infrastructure
-enable-checkov Enable the checkov policy
-enable-costs   Enable the infracost costs
-init           Runs the terraform initialization
-plan           Runs a terraform plan on the infrastructure
-var-file       Sets the local of a terraform variables file
-h,--help       Prints help message
EOF

  if [[ -n $@ ]]; then
    error "$@"
    exit 1
  fi
  exit 0
}

# validate is called to check we have all the required state
validate() {
  if [[ ${ENABLE_CHECKOV} == "true" ]]; then
    if [[ -z ${CHECKOV_POLICY_URL} ]]; then
      fatal "CHECKOV_POLICY_URL is not set"
    fi
  fi
  if [[ ${ENABLE_INFRACOST} == "true" ]]; then
    if [[ -z ${INFRACOST_API_KEY} ]]; then
      fatal "INFRACOST_API_KEY is not set"
    fi
  fi
  if [[ ${STAGE} != "plan" && ${STAGE} != "apply" && ${STAGE} != "init" && ${STAGE} != "destroy" ]]; then
    fatal "STAGE must be either 'plan', 'apply', 'destroy' or 'init'"
  fi

  return 0
}

terraform_verify() {
  echo
  echo "--------------------------------------------------------------------------------"
  echo " Evaluating Against Policy"
  echo "--------------------------------------------------------------------------------"
  echo
  announce "Verifying the plan against permitted policy"
}

terraform_cost() {
  echo
  echo "--------------------------------------------------------------------------------"
  echo " Evaluating Cost"
  echo "--------------------------------------------------------------------------------"
  echo

  INFRACOST_OPTS=""
  if [[ -f infracost-usage.yml ]]; then
    INFRACOST_OPTS="${INFRACOST_OPTS} --usage-file infracost-usage.yml"
  fi

  # @step: show the user the breakdown of the costs
  if ! infracost breakdown ${INFRACOST_OPTS} --path $TERRAFORM_PLAN_JSON .; then
    error "Unable to assess the costs of the configuration"
    exit 1
  fi

  # @show: retrieve the json from the infracost
  if ! infracost breakdown ${INFRACOST_OPTS} --path ${TERRAFORM_PLAN_JSON} --format json >${COST_REPORT_FILE}; then
    error "Unable to retrieve the json report from infracost"
    exit 1
  fi

  if ! kubectl -n ${KUBE_NAMESPACE} delete secret ${COST_REPORT_NAME} --ignore-not-found >/dev/null; then
    error "Failed to delete the cost report secret"
    exit 1
  fi

  if ! kubectl -n ${KUBE_NAMESPACE} create secret generic ${COST_REPORT_NAME} --from-file=${COST_REPORT_FILE} --validate=false >/dev/null; then
    error "Failed to create the cost report secret"
    exit 1
  fi
}

terraform_init() {
  ${TERRAFORM} init || exit 1
}

terraform_destroy() {
  ${TERRAFORM} destroy -auto-approve || exit 1
}

terraform_plan() {
  $TERRAFORM plan ${TERRAFORM_VARIABLES} -out=${TERRAFORM_PLAN} -lock=true || exit 1
  $TERRAFORM show -json $TERRAFORM_PLAN >$TERRAFORM_PLAN_JSON
  [[ ${ENABLE_CHECKOV} == "true" ]] && terraform_verify
  [[ ${ENABLE_INFRACOST} == "true" ]] && terraform_cost
}

terraform_apply() {
  ${TERRAFORM} apply ${TERRAFORM_VARIABLES} -auto-approve -lock=true || exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -apply)
      STAGE="apply"
      shift
      ;;
    -checkov-url)
      CHECKOV_POLICY_URL="$2"
      shift 2
      ;;
    -cost-api)
      INFRACOST_API_URL="$2"
      shift 2
      ;;
    -cost-token)
      INFRACOST_API_KEY="$2"
      shift 2
      ;;
    -destroy)
      STAGE="destroy"
      shift
      ;;
    -enable-checkov)
      ENABLE_CHECKOV="true"
      shift
      ;;
    -enable-costs)
      ENABLE_INFRACOST="true"
      shift
      ;;
    -init)
      STAGE="init"
      shift
      ;;
    -plan)
      STAGE="plan"
      shift
      ;;
    -var-file)
      TERRAFORM_VARIABLES="-var-file=$2"
      shift 2
      ;;
    -h | --help)
      usage
      ;;
    *)
      usage "Unknown argument: $1"
      ;;
  esac
done

# @step: first we check we've been passed all the required variables
validate || failed "Unable to validate the configuration"

echo "--------------------------------------------------------------------------------"
echo " RUNNING TERRAFORM $(echo ${STAGE} | tr '[:lower:]' '[:upper:]')"
echo "--------------------------------------------------------------------------------"

# @step: next be either perform a plan or apply
if [[ ${STAGE} == "apply" ]]; then
  terraform_apply
elif [[ ${STAGE} == "plan" ]]; then
  terraform_plan
elif [[ ${STAGE} == "destroy" ]]; then
  terraform_destroy
elif [[ ${STAGE} == "init" ]]; then
  terraform_init
else
  fatal "Unknown stage: ${STAGE}"
fi

exit 0
