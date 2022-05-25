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

load ../../lib/helper.bash

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should be able to create a blocking modules constaint" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: denied
spec:
  constraints:
    modules:
      allowed:
        - "https://none*"
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get policy denied"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete policy override || true"
  [[ "$status" -eq 0 ]]
}

@test "We should not be allowed to create a configuration" {
cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
  annotations:
    terraform.appvia.io/reconcile: "false"
spec:
  module: https://does_not_matter
  providerRef:
    namespace: terraform-system
    name: fake
EOF

  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml 2>&1" "grep 'configuration has been denied by policy'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a namespace override to the module constraint" {
  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: override
spec:
  constraints:
    modules:
      allowed:
        - "https://does_not_matter.*"
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get policy override"
  [[ "$status" -eq 0 ]]
}

@test "We should be allowed to create the configuration" {
cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
  annotations:
    terraform.appvia.io/reconcile: "false"
spec:
  module: https://does_not_matter
  providerRef:
    namespace: terraform-system
    name: fake
EOF

  runit "kubectl -n ${APP_NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the configuration" {
  runit "kubectl -n ${APP_NAMESPACE} delete configuration ${RESOURCE_NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the module constraints" {
  runit "kubectl delete policy denied"
  [[ "$status" -eq 0 ]]
  runit "kubectl delete policy override"
  [[ "$status" -eq 0 ]]
}
