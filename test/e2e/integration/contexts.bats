#!/usr/bin/env bats
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

load ../lib/helper

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to retrieve a list of contexts" {
  runit "kubectl get contexts.terraform.appvia.io"
  [[ "${status}" -eq 0 ]]
}

@test "We should be able to create a configuration context" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: terraform.appvia.io/v1alpha1
kind: Context
metadata:
  name: default
spec:
  variables:
    my_sentence:
      description: "hello world"
      value: true
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "${status}" -eq 0 ]]
  runit "kubectl get contexts.terraform.appvia.io default"
  [[ "${status}" -eq 0 ]]
}

@test "We should be able to update the terranetes context" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: terraform.appvia.io/v1alpha1
kind: Context
metadata:
  name: default
spec:
  variables:
    my_sentence:
      description: This is a description
      value: We expect to see this
    updated:
      description: "updated"
      value: true
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "${status}" -eq 0 ]]
  runit "kubectl get contexts.terraform.appvia.io default"
  [[ "${status}" -eq 0 ]]
}

@test "We should be able to use a context within a configuration" {
  namespace="context-check"

  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    kubernetes.io/metadata.name: default
  name: ${namespace}
EOF

  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "${status}" -eq 0 ]]
  runit "kubectl -n ${namespace} delete job --all"
  [[ "${status}" -eq 0 ]]
  runit "kubectl -n ${namespace} delete pod --all"
  [[ "${status}" -eq 0 ]]

  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
  providerRef:
    name: ${CLOUD}
  writeConnectionSecretToRef:
    name: custom-secret
  valueFrom:
    - context: default
      name: sentence
      key: my_sentence
  variables:
    unused: $(date +"%s")
EOF
  runit "kubectl -n ${namespace} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have successfully ran the terraform plan" {
  namespace="context-check"
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 30 "kubectl -n ${namespace} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | grep -q Complete"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${namespace} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to view the logs and see expected output" {
  namespace="context-check"
  labels="-l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan"

  POD=$(kubectl -n ${namespace} get pod ${labels} -o json | jq -r '.items[0].metadata.name')
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${namespace} logs ${POD} 2>&1" "grep -q 'We expect to see this'"
  [[ "$status" -eq 0 ]]
}

@test "We should not be able to delete context when in use" {
  runit "kubectl delete contexts.terraform.appvia.io default 2>&1" "grep -q 'resource in use by configuration'"
  [[ "${status}" -eq 0 ]]
}

@test "We should be able to delete a configuration" {
  namespace="context-check"

  runit "kubectl -n ${namespace} delete configuration ${RESOURCE_NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the configuration context" {
  runit "kubectl delete contexts.terraform.appvia.io default"
  [[ "${status}" -eq 0 ]]
  runit "kubectl get contexts.terraform.appvia.io default 2>&1" "grep -q NotFound"
  [[ "${status}" -eq 0 ]]
}
