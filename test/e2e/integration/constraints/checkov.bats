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

@test "We should be able to create a checkov policy" {
  runit "kubectl delete policy checkov || true"
  [[ "$status" -eq 0 ]]

  cat <<EOF > ${BATS_TMPDIR}/resource.yaml
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: checkov
spec:
  constraints:
    checkov:
      external:
        - name: test
          url: https://github.com/appvia/terraform-controller.git//test/e2e/assets/checkov?ref=e2e_custom_checks
EOF
  runit "kubectl apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl get policy checkov"
  [[ "$status" -eq 0 ]]
}

