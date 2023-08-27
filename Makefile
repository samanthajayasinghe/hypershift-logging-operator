include boilerplate/generated-includes.mk

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update


# -------------------------
##@ Development Environment
# -------------------------

## Installs all project dependencies into $(PWD)/.deps/bin
dependencies:
	./mage dependency:all
.PHONY: dependencies


## Setup a local env for feature development. (Kind, OLM, OKD Console)
dev-setup:
	./mage dev:setup
.PHONY: dev-setup


## Deletes the previously created kind cluster.
delete-kind-cluster:
	./mage dev:teardown
.PHONY: delete-kind-cluster
