package controller

const (
	// ServiceLabel *must* be set to 'true' on a corev1.Service that should be watched by the Controller
	ServiceLabel = "thanos.receive.hashring.controller/watch"
	// HashringNameIdentifierLabel is an optional label that is used by the controller to identify the hashring name.
	// A missing/empty value defaults to the name of the Service.
	HashringNameIdentifierLabel = "hashring.controller.io/hashring"
	// TenantIdentifierLabel is an optional label that is used by the controller to identify a tenant for the hashring
	// When relying on default behaviour for the controller, the absence of this label
	// on a Service will result in an empty tenant list which matches all tenants providing soft-tenancy
	TenantIdentifierLabel = "hashring.controller.io/tenants"
	// AlgorithmIdentifierLabel is the label that is used by the controller to identify the hashring algorithm
	// When relying on default behaviour for the controller, the absence of this label
	// on a Service will result in the use of config.DefaultAlgorithm
	AlgorithmIdentifierLabel = "hashring.controller.io/hashing-algorithm"

	// DefaultConfigMapName is the default name for the generated ConfigMap
	DefaultConfigMapName = "hashring-controller-generated-config"
	// ConfigMapLabel is the label that is used to identify configmaps that is managed by the controller
	ConfigMapLabel = "hashring.controller.io/managed"

	// configMapKey is the default key for the generated ConfigMap
	configMapKey         = "hashring.json"
	defaultPort          = "10901"
	defaultClusterDomain = "cluster.local"
)
