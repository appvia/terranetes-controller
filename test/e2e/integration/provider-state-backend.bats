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

@test "We should be able to create a cloud credential" {
  [[ ${CLOUD} == "aws" ]] && skip "no need to provision the cloud credential"

  if kubectl -n ${NAMESPACE} get secret aws; then
    skip "Cloud credential already exists"
  fi

  runit "kubectl -n ${NAMESPACE} create secret generic aws --from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} --from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} --from-literal=AWS_REGION=${AWS_REGION} >/dev/null 2>&1"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${NAMESPACE} get secret aws"
  [[ $status -eq 0   ]]
}

@test "We should be able to delete all resource before checking custom provider state" {
  runit "kubectl -n ${APP_NAMESPACE} delete jobs --all"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} delete pods --all"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} delete configurations --all"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} delete cloudresources --all"
  [[ $status -eq 0   ]]
}

@test "We should be able to create a custom backend configuration secret" {
  runit "kubectl -n terraform-system delete secret terraform-backend-config || true"
  [[ $status -eq 0   ]]
  cat << EOF > ${BATS_TMPDIR}/resource.yaml 2> /dev/null
terraform {
  backend "s3" {
    bucket     = "terranetes-controller-state-e2e-bucket"
    key        = "${GITHUB_RUN_ID:-test}/{{ .namespace }}/{{ .name }}"
    region     = "eu-west-2"
    access_key = "${AWS_ACCESS_KEY_ID}"
    secret_key = "${AWS_SECRET_ACCESS_KEY}"
  }
}
EOF
  runit "kubectl -n terraform-system create secret generic terraform-backend-config --from-file=backend.tf=${BATS_TMPDIR}/resource.yaml 2>/dev/null"
  [[ $status -eq 0   ]]
}

@test "We should be able to create a provider using custom state" {
  cat << EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Provider
metadata:
  name: provider-state
spec:
  source: secret
  provider: aws
  secretRef:
    namespace: terraform-system
    name: aws
  backendTemplate:
    namespace: terraform-system
    name: terraform-backend-config
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ $status -eq 0   ]]
  runit "kubectl get provider ${CLOUD}"
  [[ $status -eq 0   ]]
}

@test "We should be able to create a configuration with a custom provider backend" {
  cat << EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
  providerRef:
    name: provider-state
  writeConnectionSecretToRef:
    name: custom-secret
EOF

  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ $status -eq 0   ]]
}

@test "We should have a completed watcher for the configuration plan" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=plan"

  retry 30 "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].status' | grep -q True"
  [[ $status -eq 0   ]]
}

@test "We should be able to watch the logs of the confuration" {
  labels="-l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan"

  POD=$(kubectl -n ${APP_NAMESPACE} get pod ${labels} -o json | jq -r '.items[0].metadata.name')
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q '\[build\] completed'"
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

@test "We should be able to approve the terraform configuration" {
  runit "kubectl -n ${APP_NAMESPACE} annotate configurations.terraform.appvia.io ${RESOURCE_NAME} \"terraform.appvia.io/apply\"=true --overwrite"
  [[ $status -eq 0   ]]
}

@test "We should have a completed watcher for the configuration apply" {
  labels="terraform.appvia.io/configuration=${RESOURCE_NAME},terraform.appvia.io/stage=apply"

  retry 30 "kubectl -n ${APP_NAMESPACE} get job -l ${labels} -o json" "jq -r '.items[0].status.conditions[0].type' | egrep -q '(Complete|SuccessCriteriaMet)'"
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

@test "We should have the custom backend defined in it's configuration secret" {
  ID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]
  SECRET_NAME="config-${ID}"

  runit "kubectl -n terraform-system get secret ${SECRET_NAME} -o json" "jq '.data[\"backend.tf\"]' -r"
  [[ $status -eq 0   ]]
}

@test "We should be able to view the logs from the terraform apply" {
  labels="-l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=apply"

  POD=$(kubectl -n ${APP_NAMESPACE} get pod ${labels} -o json | jq -r '.items[0].metadata.name')
  [[ $status -eq 0   ]]
  runit "kubectl -n ${APP_NAMESPACE} logs ${POD} 2>&1" "grep -q '\[build\] completed'"
  [[ $status -eq 0   ]]
}

@test "We should have a terraform state file in the remote state bucket" {
  BUCKET="terranetes-controller-state-e2e-ci"
  STATE_KEY="${GITHUB_RUN_ID:-test}/${APP_NAMESPACE}/${RESOURCE_NAME}"

  runit "aws s3api head-object --bucket ${BUCKET} --key ${STATE_KEY}"
  [[ $status -eq 0   ]]
}

@test "We should be able to delete the terraform state from bucket" {
  BUCKET="terranetes-controller-state-e2e-ci"

  runit "aws s3 rm s3://${BUCKET} --recursive"
  [[ $status -eq 0   ]]
}

@test "We should have a terraform state secret in the terraform-system namespace" {
  ID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ $status -eq 0   ]]
  SECRET_NAME="tfstate-default-${ID}"

  runit "kubectl -n terraform-system get secret ${SECRET_NAME}"
  [[ $status -eq 0   ]]
}

@test "We should have a application secret in the configuration namespace" {
  runit "kubectl -n ${APP_NAMESPACE} get secret custom-secret"
  [[ $status -eq 0   ]]
}

@test "We should have the configuration secret in the application namespace" {
  runit "kubectl -n ${APP_NAMESPACE} get secret custom-secret -o json" "jq .data.NUMBER | grep -q -v null"
  [[ $status -eq 0   ]]
}

@test "We should be able to delete the configuration" {
  ID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')

  runit "kubectl -n ${APP_NAMESPACE} delete configuration ${RESOURCE_NAME}"
  [[ $status -eq 0   ]]
  runit "kubectl -n terraform-system get secret config-${ID} 2>&1" "grep -qi 'not found'"
  [[ $status -eq 0   ]]
  runit "kubectl -n terraform-system get secret tfstate-default-${ID} 2>&1" "grep -qi 'not found'"
  [[ $status -eq 0   ]]
}

@test "We should be able to delete the custom provider" {
  runit "kubectl delete provider provider-state"
  [[ $status -eq 0   ]]
}
