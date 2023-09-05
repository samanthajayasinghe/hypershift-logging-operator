# Development Guide

## Running locally

This method is often lower lift since it doesn't require a container image to be built and pushed. The operator is
run locally (i.e. with `go run .`) and depends on local K8s to interact with a K8s cluster.

1. Run the operator

    ```bash
    # We currently have no webhooks anyway, so ENABLE_WEBHOOKS=false is optional
    make run ENABLE_WEBHOOKS=false
   
    # Running with debug logs enabled
    make run ARGS="--zap-log-level=debug" ENABLE_WEBH
