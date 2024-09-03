#!/usr/bin/env bats
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

load ../lib/helper

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n $BATS_TEST_COMPLETED   ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should skip infracost when not running on aws cloud" {
  [[ -z ${INFRACOST_API_KEY}   ]] && touch ${BATS_PARENT_TMPNAME}.skip
  [[ ${CLOUD} == "aws"   ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should have a job created in the terraform-system running the plan" {
  labels="terraform.appvia.io/configuration=compute,terraform.appvia.io/stage=plan"

  runit "kubectl -n ${NAMESPACE} get job -l ${labels}"
  [[ $status -eq 0   ]]
}

@test "We should see the terraform plan complete successfully" {
  labels="terraform.appvia.io/configuration=compute,terraform.appvia.io/stage=plan"

  retry 50 "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should the predicted costs available on the status" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration compute -o json" "jq -r '.status.costs.enabled' | grep -q 'true'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration compute -o json" "jq -r '.status.costs.monthly' | grep -q '^\$[0-9\.]*'"
  [[ $status -eq 0   ]]
}

@test "We should see infracost breakdown in the watcher logs" {
  [[ -z ${INFRACOST_API_KEY}   ]] && touch ${BATS_PARENT_TMPNAME}.skip

  POD=$(kubectl -n ${APP_NAMESPACE} get pod -l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan -o json | jq -r '.items[0].metadata.name')
  [[ $status -eq 0   ]]

  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q 'EVALUATING THE COSTS'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q 'OVERALL TOTAL'"
  [[ $status -eq 0   ]]
}

@test "We should be able to destroy the aws configuration for costs" {
  runit "kubectl -n ${APP_NAMESPACE} delete -f ${BATS_TMPDIR}/resource.yml"
  [[ $status -eq 0   ]]
}
