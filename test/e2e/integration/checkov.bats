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
  [[ "${CLOUD}" == "aws" ]] || skip "skip for non-aws cloud"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should clean the environment before running the tests" {
  runit "kubectl -n ${APP_NAMESPACE} delete po --all"
  [[ "${status}" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete jobs --all"
  [[ "${status}" -eq 0 ]]
}

@test "We should be able to create a checkov policy to block resources" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: denied
spec:
  constraints:
    checkov:
      checks: []
      skipChecks: []
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get policies.terraform.appvia.io denied"
  [[ "$status" -eq 0 ]]
}

@test "We should be create a configuration to verify the policy blocks" {
  cat <<EOF >> ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?ref=v3.1.0
  providerRef:
    name: aws
  variables:
    unused: $(date +"%s")
    bucket_name: ${RESOURCE_NAME}
EOF
  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a job created in the terraform namespace running the plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n ${NAMESPACE} get job -l ${labels}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a watcher job created in the configuration namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a completed watcher job in the application namespace" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 10 "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | grep -q Complete"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}

@test "We should have a secret containing the evaluation in the terraform namespace" {
  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get secret policy-${UUID}"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get secret policy-${UUID} -o json" "jq -r '.data.results_json.json'"
  [[ "$status" -eq 0 ]]
}

@test "We should see the conditions indicate the configuration failed policy" {
  POD=$(kubectl -n ${APP_NAMESPACE} get pod -l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan -o json | jq -r '.items[0].metadata.name')
  [[ "$status" -eq 0 ]]

  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q 'EVALUATING AGAINST SECURITY POLICY'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q 'FAILED for resource'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a event indicating the configuration failed policy" {
  expected="Configuration has failed security policy, refusing to continue"

  runit "kubectl -n ${APP_NAMESPACE} get event" "grep -q 'Configuration has failed security policy, refusing to continue'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to cleanup the environment" {
  runit "kubectl -n ${APP_NAMESPACE} delete configuration ${RESOURCE_NAME}"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete po --all"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete policy denied"
  [[ "$status" -eq 0 ]]
}
