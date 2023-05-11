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

@test "We should create a namespace for testing the error handling" {
  NAMESPACE="error-handling"

  if kubectl get namespace ${NAMESPACE}; then
    skip "namespace already exists"
  fi

  runit "kubectl create namespace ${NAMESPACE}"
  [ "$status" -eq 0 ]
}

@test "We should be able to create a failing configuration" {
  NAMESPACE="error-handling"

  cat <<EOF >> ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?v3.1.0
  providerRef:
    name: ${CLOUD}
  variables:
    unused: $(date +"%s")
    bucket_name: ${RESOURCE_NAME}
EOF
  runit "kubectl -n ${NAMESPACE} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to see the logs from the failing resource" {
  NAMESPACE="error-handling"
  LABELS="-l terraform.appvia.io/configuration=${RESOURCE_NAME} -l terraform.appvia.io/stage=plan"

  retry 10 "kubectl -n ${NAMESPACE} get po --no-headers ${LABELS} -o json | wc -l | grep -v 0"
  [[ "$status" -eq 0 ]]
  POD=$(kubectl -n ${NAMESPACE} get pod ${LABELS} -o json | jq -r '.items[0].metadata.name')
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${NAMESPACE} logs ${POD} 2>&1" "grep -q 'failed to download the source'"
  [[ "$status" -eq 0 ]]
}

