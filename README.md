# hashring-controller

**This project is in early development and is not ready for production use.**

A controller to manage Thanos Receive hashring configurations.

The goal of this project is to provide the ability to have close to real time updates to the hashring configuration
in the Thanos receive replicas.

In order to achieve this, it works in two modes of operation:

1. The controller watches EndpointSlice resource via headless Service labels and sets the hashring configuration
   in a ConfigMap.
2. The controller, running in "sync" mode, sits as a sidecar in the Thanos Receive Pod and watches the ConfigMap
   for changes. When a change is detected, it will update the hashring configuration in the Thanos Receive replica.

# Testing

Run tests with `make test`

Integration tests depend on the controller-runtime [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) package.
You can install the binaries to run these tests via `go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest`
After running `setup-envtest use` move the binaries to `/usr/local/kubebuilder/bin/` or set the `KUBEBUILDER_ASSETS`
environment variable to the location of the binaries.


# Inspiration

This project is heavily inspired by the following two projects:

1. [thanos-receive-controller](https://github.com/observatorium/thanos-receive-controller) project.
2. [configmap-to-disk](https://github.com/squat/configmap-to-disk)

