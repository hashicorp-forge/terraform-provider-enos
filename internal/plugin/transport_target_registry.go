// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"sync"
)

// transportTargetRegistryKey is the context key used to store and retrieve the
// provider-level transportTargetRegistry from a context.Context.
type transportTargetRegistryKey struct{}

// withTransportTargetRegistry returns a new context with the given registry stored under
// transportTargetRegistryKey. The registry is injected by the resource router before
// ApplyResourceChange so that transport_resource_util and failure handlers can access it
// without any per-resource boilerplate.
func withTransportTargetRegistry(ctx context.Context, r *transportTargetRegistry) context.Context {
	return context.WithValue(ctx, transportTargetRegistryKey{}, r)
}

// transportTargetRegistryFromContext retrieves the registry from ctx, returning nil if none
// was stored.
func transportTargetRegistryFromContext(ctx context.Context) *transportTargetRegistry {
	r, _ := ctx.Value(transportTargetRegistryKey{}).(*transportTargetRegistry)
	return r
}

// transportTargetRegistry is a thread-safe store of resolved transport targets that have been
// configured by resources during apply. It is held on the Provider so that failure handlers
// triggered by any single resource can attempt to gather logs from every known target, not only
// the one that failed.
type transportTargetRegistry struct {
	mu      sync.Mutex
	targets []transportState
}

func newTransportTargetRegistry() *transportTargetRegistry {
	return &transportTargetRegistry{}
}

// register adds a resolved transport state. Registrations are append-only; re-applying the
// same resource simply adds another entry, which is fine because log collection is best-effort.
func (r *transportTargetRegistry) register(transport transportState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.targets = append(r.targets, transport)
}

// all returns a snapshot copy of every registered transport state. The returned slice is safe to
// iterate without holding the lock.
func (r *transportTargetRegistry) all() []transportState {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.targets) == 0 {
		return nil
	}

	out := make([]transportState, len(r.targets))
	copy(out, r.targets)

	return out
}
