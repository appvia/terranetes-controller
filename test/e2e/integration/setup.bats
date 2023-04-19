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

@test "We should be able to setup the helm repository" {
  runit "helm repo add appvia https://terranetes-controller.appvia.io"
  [[ "$status" -eq 0 ]]
  runit "helm repo update"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to deploy the helm chart" {
  CHART="charts/terranetes-controller"

  if [[ ${USE_CHART} == true ]]; then
    cat <<EOF > ${BATS_TMPDIR}/my_values.yaml
replicaCount: 1
controller:
  enableNamespaceProtection: true
  images:
    controller: "ghcr.io/appvia/terranetes-controller:${VERSION}"
    executor: "ghcr.io/appvia/terranetes-executor:${VERSION}"
  costs:
    secret: infracost-api
EOF
  else
    CHART="appvia/terranetes-controller"

    cat <<EOF > ${BATS_TMPDIR}/my_values.yaml
controller:
  enableNamespaceProtection: true
  costs:
    secret: infracost-api
EOF
  fi

  runit "helm upgrade --install terranetes-controller ${CHART} -n ${NAMESPACE} --create-namespace --values ${BATS_TMPDIR}/my_values.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should see the custom resource types" {
  runit "kubectl get crd configurations.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
  runit "kubectl get crd policies.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
  runit "kubectl get crd providers.terraform.appvia.io"
  [[ "$status" -eq 0 ]]
}

@test "We should have the controller webhooks enabled" {
  runit "kubectl get validatingwebhookconfigurations validating-webhook-configuration"
  [[ "$status" -eq 0 ]]
}

@test "We should have the terranetes-controller helm chart deployed" {
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

  runit "kubectl delete policies.terraform.appvia.io --all"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} delete job --all"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} delete po -l terraform.appvia.io/configuration"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete job --all --wait=false"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete po --all --wait=false"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete ev --all --wait=false"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to provision a secret with infracost api token" {
  [[ "${INFRACOST_API_KEY}" == "" ]] && skip "INFRACOST_API_KEY is not set"

  if kubectl -n ${NAMESPACE} get secret infracost-api; then
    skip "infracost token already exists"
  fi

  runit "kubectl -n ${NAMESPACE} create secret generic infracost-api --from-literal=INFRACOST_API_KEY=${INFRACOST_API_KEY}"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get secret infracost-api"
  [[ "$status" -eq 0 ]]
}
