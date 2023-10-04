include boilerplate/generated-includes.mk

OPERATOR_NAME=hypershift-logging-operator

SHELL := /usr/bin/env bash
CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)

# Verbosity
AT_ = @
AT = $(AT_$(V))
# /Verbosity

GIT_HASH := $(shell git rev-parse --short=7 HEAD)
PKO_IMAGETAG ?= ${GIT_HASH}

PKO_BASE_IMG ?= hypershift-logging-operator-package
PKO_IMG_REGISTRY ?= quay.io
PKO_IMG_ORG ?= app-sre
PKO_IMG ?= $(PKO_IMG_REGISTRY)/$(PKO_IMG_ORG)/${PKO_BASE_IMG}

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

.PHONY: run
run:
	go run ./main.go $(ARGS)

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	oc apply -f ./deploy/crds/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	oc delete --ignore-not-found=$(ignore-not-found) -f ./deploy/crds/	

.PHONY: build-package-push
build-package-push: 
	build/build_package.sh ${PKO_IMG}:${PKO_IMAGETAG}

.PHONY: build-package
build-package:
		$(CONTAINER_ENGINE) build -t $(PKO_IMG):$(PKO_IMAGETAG) -f $(join $(CURDIR),/hack/hypershift/package/hypershift-logging-operator.Containerfile) . && \
		$(CONTAINER_ENGINE) tag $(PKO_IMG):$(PKO_IMAGETAG) $(PKO_IMG):latest

.PHONY: skopeo-push
skopeo-push-package:
	@if [[ -z $$QUAY_USER || -z $$QUAY_TOKEN ]]; then \
		echo "You must set QUAY_USER and QUAY_TOKEN environment variables" ;\
		echo "ex: make QUAY_USER=value QUAY_TOKEN=value $@" ;\
		exit 1 ;\
	fi
	# QUAY_USER and QUAY_TOKEN are supplied as env vars
	skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${PKO_IMG}:${PKO_IMAGETAG}" \
		"docker://${PKO_IMG}:latest"
	skopeo copy --dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${PKO_IMG}:${PKO_IMAGETAG}" \
		"docker://${PKO_IMG}:${PKO_IMAGETAG}"
