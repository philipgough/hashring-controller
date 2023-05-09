# hashring-controller



# Testing

Run tests with `make test`

Integration tests depend on the controller-runtime [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) package.
You can install the binaries to run these tests via `go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest`
After running `setup-envtest use` move the binaries to `/usr/local/kubebuilder/bin/` or set the `KUBEBUILDER_ASSETS`
environment variable to the location of the binaries.
