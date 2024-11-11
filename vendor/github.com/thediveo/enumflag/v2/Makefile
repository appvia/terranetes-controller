.PHONY: help clean coverage pkgsite report test vuln chores

help: ## list available targets
	@# Shamelessly stolen from Gomega's Makefile
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

clean: ## cleans up build and testing artefacts
	rm -f coverage.*

coverage: ## gathers coverage and updates README badge
	@scripts/cov.sh

pkgsite: ## serves Go documentation on port 6060
	@echo "navigate to: http://localhost:6060/github.com/thediveo/enumflag/v2"
	@scripts/pkgsite.sh

report: ## run goreportcard on this module
	@scripts/goreportcard.sh

test: ## run unit tests
	go test -v -p=1 -race ./...

vuln: ## runs govulncheck
	@scripts/vuln.sh

chores: ## updates Go binaries and NPM helper packages if necessary
	@scripts/chores.sh
