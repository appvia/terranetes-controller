# Copyright (C) 2023  Appvia Ltd <info@appvia.io>
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; either version 2
# of the License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.


load ../lib/helper

setup() {
  [[ ! -f ${BATS_PARENT_TMPNAME}.skip ]] || skip "skip remaining tests"
}

teardown() {
  [[ -n "$BATS_TEST_COMPLETED" ]] || touch ${BATS_PARENT_TMPNAME}.skip
}

@test "We should have a validation webhook for namespaces" {
  runit "kubectl get validatingwebhookconfigurations validating-webhook-namespace"
  [[ "$status" -eq 0 ]]
}

@test "We should not be able to delete the namespace while configuration exists" {
  expected="ensure Terranetes Configurations are deleted first"

  runit "kubectl delete namespace ${APP_NAMESPACE} 2>&1" "grep -q '${expected}'"
  [[ "$status" -eq 0 ]]
}
