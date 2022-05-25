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

load ../../lib/helper

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to create a fake provider" {
  runit "kubectl -n ${NAMESPACE} delete provider fake || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} delete secret fake || true"
  [[ "$status" -eq 0 ]]

  runit "kubectl -n ${NAMESPACE} create secret generic fake --from-literal=AWS_ACCESS_KEY_ID=test --from-literal=AWS_SECRET_ACCESS_KEY=test"
  [[ "$status" -eq 0 ]]

  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Provider
metadata:
  name: fake
  namespace: ${NAMESPACE}
spec:
  source: secret
  provider: aws
  secretRef:
    namespace: terraform-system
    name: fake
EOF
  runit "kubectl -n ${NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get provider fake"
  [[ "$status" -eq 0 ]]
}

@test "We should have a healthly fake provider" {
  runit "kubectl -n ${NAMESPACE} get provider fake -o json" "jq -r '.status.conditions[0].name' | grep -q 'Provider Ready'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get provider fake -o json" "jq -r '.status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}

@test "We should have a clean application namespace before starting" {
  runit "kubectl -n ${APP_NAMESPACE} delete po --all"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} delete job --all"
  [[ "$status" -eq 0 ]]
}
