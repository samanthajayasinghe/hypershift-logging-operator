# Hypershift Audit Logs Forwarder

Hypershift logging forwarder is an operator that lives in each hosted control plane (HCP) namespace in management clusters.
The Hypershift Logging Operator combines data-plane and control-plane configuration on the control plane, so that the Cluster Logging Operator on the management cluster can forward API audit logs for both personas. 


## Development


### Prerequisites and Dependencies

To contribute new features or add/run tests, `podman` or `docker` and the `go` tool chain need to be present on the system.

Dependencies are loaded as required and are kept local to the project in the `.cache` directory and you can setup or update all dependencies via `make dependencies`

Updating dependency versions at the top of the `Makefile` will automatically update the dependencies when they are required.

If both `docker` and `podman` are present you can explicitly force the container runtime by setting `CONTAINER_RUNTIME`.

e.g.:
```
CONTAINER_RUNTIME=docker make dev-setup
```

### Deploy to local Kind cluster 

Following command will setup local dev env with [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) cluster 

```shell
make dev-setup
```

This command will:
1. Setup a cluster via kind
2. Compile your code
3. Build containers
4. Load them into the kind cluster (no registry needed)
5. Install the Hypershift Logging Forwarder Operator

This will give you a quick environment for playing with the operator.


### Delete local kind cluster 
```shell
make delete-kind-cluster
```
