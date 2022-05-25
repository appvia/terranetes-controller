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

@test "We should be able to deploy the helm chart" {
  cat <<EOF > ${BATS_TMPDIR}/my_values.yaml
replicaCount: 1
controller:
  images:
    controller: "quay.io/appvia/terraform-controller:ci"
    executor: "quay.io/appvia/terraform-executor:ci"
EOF

  if ! helm -n ${NAMESPACE} ls | grep terraform-controller; then
    runit "helm install terraform-controller charts/ -n ${NAMESPACE} --create-namespace --values ${BATS_TMPDIR}/my_values.yaml"
    [[ "$status" -eq 0 ]]
  else
    runit "helm upgrade terraform-controller charts/ -n ${NAMESPACE} --values ${BATS_TMPDIR}/my_values.yaml"
    [[ "$status" -eq 0 ]]
  fi
}

@test "We should see the custom resource types" {
  runit "kubectl get crd configurations.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
  runit "kubectl get crd policies.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
  runit "kubectl get crd providers.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
}

@test "We should have the terraform-controller helm chart deployed" {
  runit "helm ls -n ${NAMESPACE}" "grep -v deployed"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a namespace for testing" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: v1
kind: Namespace
metadata:
  labels:
    kubernetes.io/metadata.name: apps
  name: ${APP_NAMESPACE}
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete job --all --wait=false"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete po --all --wait=false"
  [[ "$status" -eq 0 ]]
}

@test "We should have a clean terraform namespace for testing" {
  labels="terraform.appvia.io/configuration=bucket,terraform.appvia.io/stage=plan"

  runit "kubectl -n ${NAMESPACE} delete job -l ${labels}"
  [[ "$status" -eq 0 ]]
}
