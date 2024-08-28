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

@test "We should have a clean environment to verify terraform variables" {
  runit "kubectl -n tfvars delete configuration ${RESOURCE_NAME} || true"
  [[ $status -eq 0   ]]
  runit "kubectl delete namespace tfvars"
  [[ $status -eq 0   ]]
  runit "kubectl create namespace tfvars || true"
  [[ $status -eq 0   ]]
}

@test "We should be create a configuration using terraform variables input" {
  cat << EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
  enableDriftDetection: true
  providerRef:
    name: ${CLOUD}
  tfVars: |
    sentence = "Hello World from E2E"
EOF
  runit "kubectl -n tfvars apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ $status -eq 0   ]]
}

@test "We should have a configuration with terraform variables" {
  runit "kubectl -n tfvars get configuration ${RESOURCE_NAME}"
  [[ $status -eq 0   ]]
}

@test "We should have a job created in the terraform-system running the plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n tfvars get job -l ${labels}"
  [[ $status -eq 0   ]]
}

@test "We should have a watcher job created in the configuration namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n tfvars get job -l ${labels}"
  [[ $status -eq 0   ]]
}

@test "We should see the terraform plan complete successfully" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 50 "kubectl -n tfvars get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n tfvars get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should have a completed watcher job in the application namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 30 "kubectl -n tfvars get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n tfvars get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}
