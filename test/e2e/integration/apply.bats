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

@test "We should be able to approve the terraform configuration" {
  runit "kubectl -n ${APP_NAMESPACE} annotate configurations.terraform.appvia.io ${RESOURCE_NAME} \"terraform.appvia.io/apply\"=true --overwrite"
  [[ $status -eq 0   ]]
}

@test "We should have a job created in the terraform-system ready to run" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=apply"

  retry 50 "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should have a job created in the application namespace ready to watch apply" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=apply"

  retry 10 "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should have a configuration sucessfully applied" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].name' | grep -q 'Terraform Apply'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].reason' | grep -q 'Ready'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].status' | grep -q 'True'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[3].type' | grep -q 'TerraformApply'"
  [[ $status -eq 0   ]]
}

@test "We should be able to view the logs from the apply" {
  POD=$(kubectl -n ${APP_NAMESPACE} get pod -l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=apply -o json | jq -r '.items[0].metadata.name')
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q '\[build\] completed'"
  [[ $status -eq 0   ]]
}

@test "We should have the terraform version on the status" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.terraformVersion' | grep -q '[0-9]\+\.[0-9]\+\.[0-9]\+'"
  [[ $status -eq 0   ]]
}
