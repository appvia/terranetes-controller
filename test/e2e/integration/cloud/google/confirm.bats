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

@test "We should have a secret in the application namespace" {
  runit "kubectl -n ${APP_NAMESPACE} get secret test"
  [[ "$status" -eq 0 ]]
}

@test "We should only have the keys specificied in the connection secret" {
  runit "kubectl -n ${APP_NAMESPACE} get secret test -o json" "jq -r .data.BUCKET_NAME"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to confirm the existence of the bucket" {
  runit "echo hello world terranetes-controller-e2e"
  [[ "$status" -eq 0 ]]
}
