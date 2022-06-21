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
#
NAME={{ .Directory }}
PWD=$(shell pwd)
UID=$(shell id -u)

.PHONY: test validate format docs

test:
	@echo "--> Testing the module"
	@$(MAKE) validate
	@$(MAKE) format
	@$(MAKE) verify
	@$(MAKE) docs

verify:
	@echo "--> Verifying against security policies"
	@docker run --rm -t \
		-v ${PWD}:/workspace \
		-w /tf bridgecrew/checkov \
		--directory /workspace \
		--framework terraform

docs:
	@echo "--> Generating documentation"
	docker run --rm -t \
		-u ${UID} \
		-v ${PWD}:/workspace \
		-w /workspace \
		quay.io/terraform-docs/terraform-docs:0.16.0 \
		markdown document --output-file README.md --output-mode inject .

validate:
	@echo "--> Validating terraform module"
	@terraform init
	@terraform validate

format:
	@echo "--> Formatting terraform module"
	@terraform fmt

controller-kind:
	@echo "--> Creating Kubernetes Cluster"
	@kind --version >/dev/null 2>&1 || (echo "ERROR: kind is required."; exit 1)
	@helm version >/dev/null 2>&1 || (echo "ERROR: helm is required."; exit 1)
	@kubectl version --client >/dev/null 2>&1 || (echo "ERROR: kubectl is required."; exit 1)
	@kind create cluster || true
	@echo "--> Adding Terranetes Helm Repository"
	@helm repo add appvia https://terraform-controller.appvia.io
	@echo "--> Deploying Terraform Controller"
	@helm upgrade -n terraform-system terraform-controller appvia/terraform-controller --create-namespace --install
	@echo "--> Terranetes Controller is available, please configure credentials"
	@echo "--> Documentation: https://terranetes.appvia.io/terraform-controller/category/administration/"
	@kubectl -n terraform-system get deployment
