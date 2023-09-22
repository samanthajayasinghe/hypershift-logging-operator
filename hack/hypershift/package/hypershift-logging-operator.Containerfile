FROM scratch
ADD deploy/crds/logging.managed.openshift.io_hypershiftlogforwarders.yaml /package/
ADD ./hack/hypershift/package/*.yaml* /package/
