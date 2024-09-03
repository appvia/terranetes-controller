
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

@test "We should be able to redeploy the controller with namespace protection disabled" {
  CHART="charts/terranetes-controller"

  if [[ "${USE_CHART}" == "false" ]]; then
    cat <<EOF > ${BATS_TMPDIR}/my_values.yaml
replicaCount: 1
controller:
  enableNamespaceProtection: false
  images:
    controller: "ghcr.io/appvia/terranetes-controller:${VERSION}"
    executor: "ghcr.io/appvia/terranetes-executor:${VERSION}"
    preload: "ghcr.io/appvia/terranetes-executor:${VERSION}"
EOF 
  if [[ "${INFRACOST_API_KEY}" != "" ]]; then
    cat <<EOF >> ${BATS_TMPDIR}/my_values.yaml
  costs:
    secret: infracost-api
EOF
  else
    CHART="appvia/terranetes-controller"

    cat <<EOF > ${BATS_TMPDIR}/my_values.yaml
controller:
  enableNamespaceProtection: false
EOF 
    if [[ "${INFRACOST_API_KEY}" != "" ]]; then
      cat <<EOF >> ${BATS_TMPDIR}/my_values.yaml
  costs:
    secret: infracost-api
EOF
    fi
  fi

  runit "helm upgrade terranetes-controller ${CHART} -n ${NAMESPACE} --values ${BATS_TMPDIR}/my_values.yaml"
  [[ "$status" -eq 0 ]]
}

@test "We should have the terranetes-controller helm chart successfully redeployed" {
  runit "helm ls -n ${NAMESPACE}" "grep -v deployed"
  [[ "$status" -eq 0 ]]
}

@test "We should no longer have the validating webhook for namespaces" {
  runit "kubectl get validatingwebhookconfigurations validating-webhook-namespace 2>&1" "grep -q 'NotFound'"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a namespace for the namespace protection checks" {
  namespace="protection"

  runit "kubectl delete namespace ${namespace} --wait=true || true"
  [[ "$status" -eq 0 ]]
  runit "kubectl create namespace ${namespace}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to create a configuration to check namespace protection" {
  namespace="protection"

cat <<EOF > ${BATS_TMPDIR}/resource.yaml
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: ${RESOURCE_NAME}
spec:
  module: https://github.com/appvia/terranetes-controller.git//test/e2e/assets/terraform/dummy?ref=master
  providerRef:
    name: ${CLOUD}
EOF

  runit "kubectl -n ${namespace} apply -f ${BATS_TMPDIR}/resource.yaml"
  [[ "$status" -eq 0 ]]
  runit "kubectl -n ${namespace} get configuration ${RESOURCE_NAME}"
  [[ "$status" -eq 0 ]]
}

@test "We should be able to delete the namespace without blocking" {
  namespace="protection"

  runit "kubectl delete namespace ${namespace} --wait=false"
  [[ "$status" -eq 0 ]]
  retry 20 "kubectl -n ${namespace} get configuration ${RESOURCE_NAME} 2>&1" "grep -q 'NotFound'"
  [[ "$status" -eq 0 ]]
}
