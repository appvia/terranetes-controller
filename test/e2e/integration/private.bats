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
  [[ -n ${E2E_SSH_DEPLOYMENT_KEY}     ]] || skip "E2E_SSH_DEPLOYMENT_KEY not set"
}

teardown() {
  [[ -n $BATS_TEST_COMPLETED   ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to create a ssh key for authentication to a private repository" {
  kubectl -n ${APP_NAMESPACE} get secret ssh && skip "ssh secret already exists"

  cat << EOF > ${BATS_TEST_DIRNAME}/ssh-key
${E2E_SSH_DEPLOYMENT_KEY}
EOF
  runit "kubectl -n ${APP_NAMESPACE} create secret generic ssh --from-file=SSH_AUTH_KEY=${BATS_TEST_DIRNAME}/ssh-key"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get secret ssh"
  [[ $status -eq 0   ]]
  runit "rm -f ${BATS_TEST_DIRNAME}/ssh-key"
  [[ $status -eq 0   ]]
}

@test "We should be able to create a configuration from a private repository" {
  cat << EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  auth:
    name: ssh
  module: git::ssh://git@github.com/appvia/terraform-private-e2e?ref=main
  providerRef:
    name: $CLOUD
EOF
  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME}"
  [[ $status -eq 0   ]]
}

@test "We should have a job created in the terraform-system running the plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n ${NAMESPACE} get job -l ${labels}"
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

@test "We should be able to delete the configuration" {
  runit "kubectl -n ${APP_NAMESPACE} delete configuration ${RESOURCE_NAME}"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} delete po --all"
  [[ $status -eq 0   ]]
}
