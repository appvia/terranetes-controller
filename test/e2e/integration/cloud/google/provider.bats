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

load ../../../lib/helper

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to create a cloud credential" {
  if kubectl -n ${NAMESPACE} get secret google; then
    skip "Cloud credential already exists"
  fi

  runit "echo ${GOOGLE_CREDENTIALS} | base64 -d | kubectl -n ${NAMESPACE} apply -f - 2>/dev/null"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get secret google"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a cloud provider" {
  runit "kubectl apply -f examples/google-provider.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get provider google"
  [[ "$status" -eq 0 ]]
}

@test "We should have a healthy cloud provider" {
  runit "kubectl get provider google -o json" "jq -r '.status.conditions[0].name' | grep -q 'Provider Ready'"
  [[ "$status" -eq 0 ]]
  runit "kubectl get provider google -o json" "jq -r '.status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}
