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
  [[ ${CLOUD} == "aws" ]] || skip "drift check only runs on aws"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should have a configuration currently insync" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.resourceStatus' | grep -q 'InSync'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} delete job --all"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete a cloud resource to simulate drift" {
  runit "aws s3 rb s3://${BUCKET}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to force a drift check via an annotation" {
  runit "kubectl -n ${APP_NAMESPACE} annotate configurations ${RESOURCE_NAME} 'terraform.appvia.io/drift'=$(date +'%s') --overwrite"
  [[ "$status" -eq 0 ]]
}

@test "We should have a job created in the terraform-system running the plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 10 "kubectl -n ${NAMESPACE} get job -l ${labels}"
  [[ "$status" -eq 0 ]]
}

@test "We should see the terraform plan complete successfully after drift" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 50 "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | grep -q Complete"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}

@test "We should see the configuration is now out of sync" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.resourceStatus' | grep -q 'OutOfSync'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to patch the configuration as auto apply and synchronize the resources again" {
  runit "kubectl -n ${APP_NAMESPACE} patch configuration ${RESOURCE_NAME} --type merge -p '{\"spec\":{\"enableAutoApproval\":true}}'"
  [[ "$status" -eq 0 ]]
}

@test "We should have an apply job created in the terraform-system namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=apply"

  retry 50 "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | grep -q Complete"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}

@test "We should see the configuration again in sync" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.resourceStatus' | grep -q 'InSync'"
  [[ "$status" -eq 0 ]]
}
