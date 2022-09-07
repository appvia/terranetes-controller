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
  if kubectl -n ${NAMESPACE} get secret ${CLOUD}; then
    skip "Cloud credential already exists"
  fi

  runit "kubectl -n ${NAMESPACE} create secret generic ${CLOUD} --from-literal=AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID} --from-literal=AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY} --from-literal=AWS_REGION=${AWS_REGION} >/dev/null 2>&1"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} get secret ${CLOUD}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a cloud provider" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Provider
metadata:
  name: ${CLOUD}
spec:
  source: secret
  provider: ${CLOUD}
  secretRef:
    namespace: terraform-system
    name: ${CLOUD}
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get provider ${CLOUD}"
  [[ "$status" -eq 0 ]]
}

@test "We should have a healthy cloud provider" {
  runit "kubectl get provider ${CLOUD} -o json" "jq -r '.status.conditions[0].name' | grep -q 'Provider Ready'"
  [[ "$status" -eq 0 ]]
  runit "kubectl get provider ${CLOUD} -o json" "jq -r '.status.conditions[0].status' | grep -q True"
  [[ "$status" -eq 0 ]]
}
