// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportTargetRegistry_Register(t *testing.T) {
	t.Parallel()

	r := newTransportTargetRegistry()

	transport := newEmbeddedTransportSSH()
	transport.Host.Set("10.0.0.1")

	r.register(transport)

	all := r.all()
	require.Len(t, all, 1)
	assert.Equal(t, transport, all[0])
}

func TestTransportTargetRegistry_RegisterMultiple(t *testing.T) {
	t.Parallel()

	r := newTransportTargetRegistry()

	ssh1 := newEmbeddedTransportSSH()
	ssh1.Host.Set("10.0.0.1")

	ssh2 := newEmbeddedTransportSSH()
	ssh2.Host.Set("10.0.0.2")

	k8s := newEmbeddedTransportK8Sv1()
	k8s.Pod.Set("vault-0")

	r.register(ssh1)
	r.register(ssh2)
	r.register(k8s)

	require.Len(t, r.all(), 3)
}

func TestTransportTargetRegistry_AllReturnsCopy(t *testing.T) {
	t.Parallel()

	r := newTransportTargetRegistry()

	transport := newEmbeddedTransportSSH()
	transport.Host.Set("10.0.0.1")
	r.register(transport)

	// Two calls should return independent slices (modifying one doesn't affect the registry).
	require.Len(t, r.all(), 1)
	require.Len(t, r.all(), 1)
}

func TestTransportTargetRegistry_ConcurrentRegister(t *testing.T) {
	t.Parallel()

	r := newTransportTargetRegistry()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()

			transport := newEmbeddedTransportSSH()
			transport.Host.Set("10.0.0." + string(rune('0'+idx%10)))
			r.register(transport)
		}(i)
	}

	wg.Wait()

	assert.Len(t, r.all(), goroutines)
}

func TestTransportTargetRegistry_EmptyRegistryAll(t *testing.T) {
	t.Parallel()

	r := newTransportTargetRegistry()
	assert.Nil(t, r.all())
}
