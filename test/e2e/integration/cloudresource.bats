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
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to create a revision" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Revision
metadata:
  name: fake.v1
spec:
  plan:
    name: fake
    description: Fake module for testing
    revision: v0.0.1
  inputs:
    - key: sentence
      description: Hello
      required: true
      default:
        value: This is the default value
  configuration:
    module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have a plan derived from the revision" {
  retry 5 "kubectl get plan fake"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a cloudresource from the plan" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: CloudResource
metadata:
  name: fake
spec:
  plan:
    name:  fake
    revision: v0.0.1
  providerRef:
    name: ${CLOUD}
  variables:
    sentence: Has been overridden
EOF
  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have a managed configuration name for derived cloudresource" {
  retry 5 "kubectl -n ${APP_NAMESPACE} get cloudresource fake -o json" "jq -r .status.configurationName | grep ^fake"
  [[ "$status" -eq 0 ]]
}

@test "We should have a configuration resource" {
  NAME=$(kubectl -n ${APP_NAMESPACE} get cloudresource fake -o json | jq -r .status.configurationName)
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a configuration in pending approval" {
  expected="Waiting for changes to be approved"

  runit "kubectl -n ${APP_NAMESPACE} get cloudresource ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[1].name' | grep -q 'ConfigurationStatus'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get cloudresource ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[1].reason' | grep -q 'InProgress'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get cloudresource ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[1].status' | grep -q 'False'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get cloudresource ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[1].type' | grep -q 'Ready'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get cloudresource ${RESOURCE_NAME} -o json" "jq -r '.status.conditions[1].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the cloudresource" {
  runit "kubectl -n ${APP_NAMESPACE} delete cloudresource fake"
  [[ "$status" -eq 0 ]]
}

@test "We should not have a cloudresource" {
  CONFIGURATION_NAME=$(kubectl -n ${APP_NAMESPACE} get cloudresource fake -o json | jq -r .status.configurationName)

  retry 10 "kubectl -n ${APP_NAMESPACE} get cloudresource fake"
  [[ "$status" -eq 1 ]]
  retry 10 "kubectl -n ${APP_NAMESPACE} get configuration ${CONFIGURATION_NAME}"
  [[ "$status" -eq 1 ]]
}

