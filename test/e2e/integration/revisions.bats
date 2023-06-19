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

@test "We should be able to list revisions" {
  runit "kubectl get revisions"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a revision" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Revision
metadata:
  name: database.v1
spec:
  plan:
    name: dummy
    categories: [mysql, aws, database]
    description: Provides a dummy terraform module for testing
    revision: v0.5.3
  inputs:
    - key: sentence
      description: Hello
      required: true
      default:
        value: hello from me
  configuration:
    module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have a plan derived from the revision" {
  retry 5 "kubectl get plan dummy"
  [[ "$status" -eq 0 ]]
}

@test "We should be the latest version on the plan" {
  runit "kubectl get plan dummy -o jsonpath='{.status.latest.version}'" "grep -q v0.5.3"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the revision" {
  runit "kubectl delete revision database.v1"
  [[ "$status" -eq 0 ]]
}

@test "We should no longer have a plan" {
  retry 10 "kubectl get plan dummy"
  [[ "$status" -eq 1 ]]
}
