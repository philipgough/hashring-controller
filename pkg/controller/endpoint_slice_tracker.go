package controller

import (
	"sync"
	"time"

	"github.com/go-kit/log"
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
