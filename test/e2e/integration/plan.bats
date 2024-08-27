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

@test "We should have a condition indicating the provider is ready" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[0].name' | grep -q 'Provider ready'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[0].status' | grep -q 'True'"
  [[ $status -eq 0   ]]
}

@test "We should have a job created in the terraform-system running the plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n ${NAMESPACE} get job -l ${labels}"
  [[ $status -eq 0   ]]
}

@test "We should have a watcher job created in the configuration namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels}"
  [[ $status -eq 0   ]]
}

@test "We should see the terraform plan complete successfully" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 50 "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should have a completed watcher job in the application namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 30 "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should have a secret containing the terraform plan" {
  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]

  runit "kubectl -n ${NAMESPACE} get secret tfplan-out-${UUID}"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get secret tfplan-out-${UUID} -o json" "jq -r '.data[\"plan.out\"]'"
  [[ $status -eq 0   ]]
}

@test "We should have a secret containing the terraform plan in json" {
  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]

  runit "kubectl -n ${NAMESPACE} get secret tfplan-json-${UUID}"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get secret tfplan-json-${UUID} -o json" "jq -r '.data[\"plan.json\"]'"
  [[ $status -eq 0   ]]
}

@test "We should have a configuration in pending approval" {
  expected="Waiting for terraform apply annotation to be set to true"

  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].name' | grep -q 'Terraform Apply'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].reason' | grep -q 'ActionRequired'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].status' | grep -q 'False'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].type' | grep -q 'TerraformApply'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].message' | grep -q '${expected}'"
  [[ $status -eq 0   ]]
}

@test "We should be able to view the logs from the plan" {
  POD=$(kubectl -n ${APP_NAMESPACE} get pod -l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan -o json | jq -r '.items[0].metadata.name')
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q '\[build\] completed'"
  [[ $status -eq 0   ]]
}

@test "We should have a secret in the terraform namespace containing the report" {
  [[ ${INFRACOST_API_KEY} == ""   ]] && skip "INFRACOST_API_KEY is not set"

  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]

  runit "kubectl -n ${NAMESPACE} get secret costs-${UUID}"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get secret costs-${UUID} -o json" "jq -r '.data[\"costs.json\"]'"
  [[ $status -eq 0   ]]
}

@test "We should see the cost integration is enabled" {
  [[ ${INFRACOST_API_KEY} == ""   ]] && skip "INFRACOST_API_KEY is not set"

  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.enabled' | grep -q true"
  [[ $status -eq 0   ]]
}

@test "We should see the cost associated to the configuration" {
  [[ ${INFRACOST_API_KEY} == ""   ]] && skip "INFRACOST_API_KEY is not set"

  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.monthly' | grep -q '\$0'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.hourly' | grep -q '\$0'"
  [[ $status -eq 0   ]]
}

@test "We should have a copy of the infracost report in the configuration namespace" {
  [[ ${INFRACOST_API_KEY} == ""   ]] && skip "INFRACOST_API_KEY is not set"

  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]

  runit "kubectl -n ${APP_NAMESPACE} get secret costs-${UUID}"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get secret costs-${UUID} -o json" "jq -r '.data[\"costs.json\"]'"
  [[ $status -eq 0   ]]
}
