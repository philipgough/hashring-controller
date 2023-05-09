package controller

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
)

// cacheKey is the cache key and the name of the hashring
// combined with the name of the headless Service which owns the EndpointSlice
// in the format <HashringName>/<Service.Name>
type cacheKey string

// ownerRefUID is the cache sub key and is made of a combination of
// the EndpointSlice OwnerReference name and the EndpointSlice UID
// in the format <EndpointSlice.Name>/<EndpointSlice.UID>
type ownerRefUID string

// hashring is a representation of Thanos hashring configuration
// which caches the endpoints and their expiry time
type hashring struct {
	tenants   []string
	endpoints map[string]*time.Time
}

// ownerRefTracker is a map of hashrings for a given OwnerReference UUID
type ownerRefTracker map[ownerRefUID]*hashring

type tracker struct {
	// ttl is the TTL for endpoints in the hashring
	// This is used to allow for involuntary disruptions to remain in the hashring
	ttl *time.Duration
	// mut is a mutex to protect the state
	mut sync.RWMutex
	// state is a map of Service name to a map of OwnerReference UUID to hashring
	state map[cacheKey]ownerRefTracker
	// now is a function that returns the current time
	// it is used to allow for testing
	now    func() time.Time
	logger log.Logger
}

func newTracker(ttl *time.Duration) *tracker {
	return &tracker{
		ttl:   ttl,
		state: make(map[cacheKey]ownerRefTracker),
		now:   time.Now,
	}
}

func (t *tracker) generateCacheKey(eps *discoveryv1.EndpointSlice) (cacheKey, error) {
	key, ok := eps.GetLabels()[discoveryv1.LabelServiceName]
	if !ok || key == "" {
		return "", fmt.Errorf("EndpointSlice %s/%s does not have a %s label",
			eps.Namespace, eps.Name, discoveryv1.LabelServiceName)
	}

	name, ok := t.getHashringName(eps)
	if !ok || name == "" {
		name = key
	}

	return cacheKey(fmt.Sprintf("%s/%s", name, key)), nil
}

// unwrapCacheKey returns the hashring name and the name of the service
func (t *tracker) unwrapCacheKey(key cacheKey) (string, string) {
	parts := strings.Split(string(key), "/")
	if len(parts) == 1 {
		return parts[0], parts[0]
	}
	return parts[0], parts[1]
}

func (t *tracker) getHashringName(eps *discoveryv1.EndpointSlice) (string, bool) {
	name, ok := eps.GetLabels()[HashringNameIdentifierLabel]
	return name, ok
}

func (t *tracker) getTenants(eps *discoveryv1.EndpointSlice) []string {
	tenant, ok := eps.GetLabels()[TenantIdentifierLabel]
	if !ok {
		return []string{}
	}
	return []string{tenant}
}

func (t *tracker) toSubKey(eps *discoveryv1.EndpointSlice) ownerRefUID {
	return ownerRefUID(fmt.Sprintf("%s/%s", eps.Name, eps.UID))
}

func (t *tracker) fromSubKey(subKey ownerRefUID) (name string, uid types.UID) {
	parts := strings.Split(string(subKey), "/")
	if len(parts) != 2 {
		// log error
		return name, uid
	}
	name = parts[0]
	uid = types.UID(parts[1])
	return name, uid
}
