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

@test "We should have a clean environment to verify cloudresources" {
  runit "kubectl delete namespace cs-apps || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete revision fake.v1 || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete revision fake.v2 || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete plan fake || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl create namespace cs-apps || true"
  [[ "$status" -eq 0 ]]
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
  runit "kubectl -n cs-apps apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have a managed configuration name for derived cloudresource" {
  retry 5 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r .status.configurationName | grep ^fake"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource with a derive configuration" {
  NAME=$(kubectl -n cs-apps get cloudresource fake -o json | jq -r .status.configurationName)
  [[ "$status" -eq 0 ]]
  runit "kubectl -n cs-apps get configuration ${NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource running the terraform plan" {
  expected="Terraform plan is running"

  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].name' | grep -q 'Terraform Plan'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].reason' | grep -q 'InProgress'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].status' | grep -q 'False'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].type' | grep -q 'TerraformPlan'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource status of out of sync" {
  retry 5 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.resourceStatus' | grep -q 'OutOfSync'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource which successfully ran a plan" {
  expected="Terraform plan is complete"

  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].name' | grep -q 'Terraform Plan'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].reason' | grep -q 'Ready'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].status' | grep -q 'True'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].type' | grep -q 'TerraformPlan'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[3].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource pending a approval" {
  expected="Waiting for terraform apply annotation to be set to true"

  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].name' | grep -q 'Terraform Apply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].reason' | grep -q 'ActionRequired'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].status' | grep -q 'False'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].type' | grep -q 'TerraformApply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to approve the cloudresource" {
  runit "kubectl -n cs-apps annotate cloudresources.terraform.appvia.io fake \"terraform.appvia.io/apply\"=true --overwrite"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource running the terraform apply" {
  expected="Terraform apply in progress"

  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].name' | grep -q 'Terraform Apply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].reason' | grep -q 'InProgress'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].status' | grep -q 'False'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].type' | grep -q 'TerraformApply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource which successfully ran a apply" {
  expected="Terraform apply is complete"

  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].name' | grep -q 'Terraform Apply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].reason' | grep -q 'Ready'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].status' | grep -q 'True'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].type' | grep -q 'TerraformApply'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.conditions[5].message' | grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource status of in sync" {
  retry 5 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.resourceStatus' | grep -q 'InSync'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to view the logs from the apply" {
  POD=$(kubectl -n cs-apps get pod -l terraform.appvia.io/configuration=fake -l terraform.appvia.io/stage=apply -o json | jq -r '.items[0].metadata.name')
  [[ "$status" -eq 0 ]]
  runit "kubectl -n cs-apps logs ${POD} 2>&1" "grep -q 'Has been overridden'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n cs-apps logs ${POD} 2>&1" "grep -q '\[build\] completed'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource with no upgrades available" {
  runit "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.updateAvailable' | grep -q 'None'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a new revision to the plan" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Revision
metadata:
  name: fake.v2
spec:
  plan:
    name: fake
    description: Fake module for testing
    revision: v0.0.2
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

@test "We should have a cloudresource pending an upgrade" {
  runit "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.updateAvailable' | grep -q 'Update v0.0.2 available'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the cloudresource" {
  runit "kubectl -n cs-apps delete cloudresource fake --wait=false"
  [[ "$status" -eq 0 ]]
}

@test "We should have a cloudresource with status deleting" {
  retry 5 "kubectl -n cs-apps get cloudresource fake -o json" "jq -r '.status.resourceStatus' | grep -q 'Deleting'"
  [[ "$status" -eq 0 ]]
}

@test "We should have deleted the cloudresource and configuration" {
  CONFIGURATION_NAME=$(kubectl -n cs-apps get cloudresource fake -o json | jq -r .status.configurationName)

  retry 10 "kubectl -n cs-apps get cloudresource fake 2>&1" "grep -q 'NotFound'"
  [[ "$status" -eq 0 ]]
  retry 10 "kubectl -n cs-apps get configuration ${CONFIGURATION_NAME} 2>&1" "grep -q 'NotFound'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the revision" {
  runit "kubectl -n cs-apps delete revision fake.v1"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n cs-apps delete revision fake.v2"
  [[ "$status" -eq 0 ]]
}

@test "We should have deleted the plan due to no revisions available" {
  retry 10 "kubectl -n cs-apps get cloudresource fake 2>&1" "grep -q 'NotFound'"
  [[ "$status" -eq 0 ]]
}
