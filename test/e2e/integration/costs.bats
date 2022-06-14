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

@test "We should have a token for the infracost api" {
  [[ -n "$INFRACOST_TOKEN" ]] || touch ${BATS_PARENT_TMPNAME}.skip
  [[ "$status" -eq 0 ]]
}

@test "We should have a secret in the terraform namespace containing the report" {
  UUID=$(kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json | jq -r '.metadata.uid')
  [[ "$status" -eq 0 ]]

  runit "kubectl -n ${NAMESPACE} get secret costs-${UUID}"
  [[ "$status" -eq 0 ]]
}

@test "We should see the cost integration is enabled" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.enabled' | grep -q true"
  [[ "$status" -eq 0 ]]
}

@test "We should see the cost associated to the configuration" {
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.monthly' | grep -q '\$0'"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${APP_NAMESPACE} get configuration ${RESOURCE_NAME} -o json" "jq -r '.status.costs.hourly' | grep -q '\$0'"
  [[ "$status" -eq 0 ]]
}

