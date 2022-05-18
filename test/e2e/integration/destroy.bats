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

@test "We should be able to delete the configuration" {
  runit "kubectl -n ${APP_NAMESPACE} delete configuration bucket --wait=false"
  [[ "$status" -eq 0 ]]
}

@test "We should have a job created in the application namespace to watch the destroy" {
  labels="terraform.appvia.io/configuration=bucket,terraform.appvia.io/stage=destroy"

  # note: we don't watch the job here it has ownership references to the configuration which
  # causes an race between the check and the deletion
  runit "kubectl -n ${APP_NAMESPACE} get job -l ${labels} --show-labels" "grep -q 'destroy'"
  [[ "$status" -eq 0 ]]
}

@test "We should have a job created in the terraform namespace to destroy the configuration" {
  labels="terraform.appvia.io/configuration=bucket,terraform.appvia.io/stage=destroy"

  runit "kubectl -n ${NAMESPACE} get job -l ${labels} --show-labels" "grep -q 'destroy'"
  [[ "$status" -eq 0 ]]
}

@test "We should not have configuration present in the application namespace" {
  retry 10 "kubectl -n ${APP_NAMESPACE} get configuration bucket 2>&1" "grep -q NotFound"
  [[ "$status" -eq 0 ]]
}

@test "We should not have the application secret present" {
  runit "kubectl -n ${APP_NAMESPACE} get secret test 2>&1" "grep -q NotFound"
  [[ "$status" -eq 0 ]]
}

@test "We should have a confirmation the bucket have been deleted" {
  expected="The specified bucket does not exist"

  runit "aws s3 ls s3://${BUCKET} 2>&1" "grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}
