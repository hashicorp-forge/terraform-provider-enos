package vault

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
)

func TestCheckStateHasSystemdEnabledAndRunningProperties(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		props      systemd.UnitProperties
		shouldFail bool
	}{
		"enabled": {
			systemd.EnabledAndRunningProperties,
			false,
		},
		"not-loaded": {
			systemd.UnitProperties{
				"LoadState":   "not-found",
				"ActiveState": "inactive",
				"SubState":    "dead",
			},
			true,
		},
		"activating": {
			systemd.UnitProperties{
				"LoadState":   "loaded",
				"ActiveState": "activating",
				"SubState":    "dead",
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.UnitProperties = test.props
			if test.shouldFail {
				require.Error(t, CheckStateHasSystemdEnabledAndRunningProperties()(state))
			} else {
				require.NoError(t, CheckStateHasSystemdEnabledAndRunningProperties()(state))
			}
		})
	}
}

func TestCheckStateAllPodsHavePhase(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		pods       kubernetes.Pods
		shouldFail bool
	}{
		"no-pods": {
			kubernetes.Pods{},
			true,
		},
		"some-pending": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-1",
						},
						Status: v1.PodStatus{
							Phase:   v1.PodPending,
							Reason:  "sigh",
							Message: "hurry up, sheesh",
						},
					},
				},
			},
			true,
		},
		"some-failed": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-1",
						},
						Status: v1.PodStatus{
							Phase:   v1.PodFailed,
							Reason:  "sometimes things happen",
							Message: "it had a bad day",
						},
					},
				},
			},
			true,
		},
		"all-running": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-1",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.PodList = &kubernetes.ListPodsResponse{Pods: &test.pods}
			if test.shouldFail {
				require.Error(t, CheckStateAllPodsHavePhase(v1.PodRunning)(state))
			} else {
				require.NoError(t, CheckStateAllPodsHavePhase(v1.PodRunning)(state))
			}
		})
	}
}

func TestCheckStatePodsHasPhase(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		pods       kubernetes.Pods
		shouldFail bool
	}{
		"no-pods": {
			kubernetes.Pods{},
			true,
		},
		"no-matching-pod": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nope",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
			},
			true,
		},
		"pending": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase:   v1.PodPending,
							Reason:  "sigh",
							Message: "hurry up, sheesh",
						},
					},
				},
			},
			true,
		},
		"failed": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase:   v1.PodFailed,
							Reason:  "sometimes things happen",
							Message: "it had a bad day",
						},
					},
				},
			},
			true,
		},
		"running": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							Phase: v1.PodRunning,
						},
					},
				},
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.PodList = &kubernetes.ListPodsResponse{Pods: &test.pods}
			if test.shouldFail {
				require.Error(t, CheckStatePodHasPhase("vault-0", v1.PodRunning)(state))
			} else {
				require.NoError(t, CheckStatePodHasPhase("vault-0", v1.PodRunning)(state))
			}
		})
	}
}

func TestCheckStateAllContainersAreReady(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		pods       kubernetes.Pods
		shouldFail bool
	}{
		"no-pods": {
			kubernetes.Pods{},
			true,
		},
		"some-not-ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: false,
								},
							},
						},
					},
				},
			},
			true,
		},
		"all-ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: true,
								},
							},
						},
					},
				},
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.PodList = &kubernetes.ListPodsResponse{Pods: &test.pods}
			if test.shouldFail {
				require.Error(t, CheckStateAllContainersAreReady()(state))
			} else {
				require.NoError(t, CheckStateAllContainersAreReady()(state))
			}
		})
	}
}

func TestCheckStateAllPodContainersAreReady(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		pods       kubernetes.Pods
		shouldFail bool
	}{
		"no-pods": {
			kubernetes.Pods{},
			true,
		},
		"no-matching-pod": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nope",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
							},
						},
					},
				},
			},
			true,
		},
		"some-not-ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: false,
								},
							},
						},
					},
				},
			},
			true,
		},
		"all-ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: true,
								},
							},
						},
					},
				},
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.PodList = &kubernetes.ListPodsResponse{Pods: &test.pods}
			if test.shouldFail {
				require.Error(t, CheckStateAllPodContainersAreReady("vault-0")(state))
			} else {
				require.NoError(t, CheckStateAllPodContainersAreReady("vault-0")(state))
			}
		})
	}
}

func TestCheckStatePodContainersIsReady(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		pods       kubernetes.Pods
		shouldFail bool
	}{
		"no-pods": {
			kubernetes.Pods{},
			true,
		},
		"no-matching-pod": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nope",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: true,
								},
							},
						},
					},
				},
			},
			true,
		},
		"no-matching-container": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
							},
						},
					},
				},
			},
			true,
		},
		"not-ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: false,
								},
							},
						},
					},
				},
			},
			true,
		},
		"ready": {
			kubernetes.Pods{
				Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "vault-0",
						},
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								{
									Name:  "vault",
									Ready: true,
								},
								{
									Name:  "vault-agent",
									Ready: true,
								},
							},
						},
					},
				},
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.PodList = &kubernetes.ListPodsResponse{Pods: &test.pods}
			if test.shouldFail {
				require.Error(t, CheckStatePodContainerIsReady("vault-0", "vault-agent")(state))
			} else {
				require.NoError(t, CheckStatePodContainerIsReady("vault-0", "vault-agent")(state))
			}
		})
	}
}

func TestCheckStateHasStatusCode(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		has        StatusCode
		wants      StatusCode
		shouldFail bool
	}{
		"is-initialized-unsealed": {
			StatusInitializedUnsealed,
			StatusInitializedUnsealed,
			false,
		},
		"not-initialized-unsealed": {
			StatusUnknown,
			StatusInitializedUnsealed,
			true,
		},
		"is-error": {
			StatusError,
			StatusError,
			false,
		},
		"not-error": {
			StatusUnknown,
			StatusError,
			true,
		},
		"is-sealed": {
			StatusSealed,
			StatusSealed,
			false,
		},
		"not-sealed": {
			StatusUnknown,
			StatusSealed,
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.Status = NewStatusResponse()
			state.Status.StatusCode = test.has
			if test.shouldFail {
				require.Error(t, CheckStateHasStatusCode(test.wants)(state))
			} else {
				require.NoError(t, CheckStateHasStatusCode(test.wants)(state))
			}
		})
	}
}

func TestCheckStateIsInitialized(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		shouldFail bool
	}{
		"is-initialized": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.Initialized = true
				s.Health = NewHealthResponse()
				s.Health.Initialized = true

				return s
			},
			false,
		},
		"no-state": {
			func() *State { return nil },
			true,
		},
		"no-status": {
			func() *State {
				s := NewState()
				s.Health = NewHealthResponse()
				s.Health.Initialized = true

				return s
			},
			true,
		},
		"no-health": {
			func() *State {
				s := NewState()
				s.Health = NewHealthResponse()
				s.Health.Initialized = true

				return s
			},
			true,
		},
		"health-and-status-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.Initialized = true
				s.Health = NewHealthResponse()
				s.Health.Initialized = false

				return s
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.shouldFail {
				require.Error(t, CheckStateIsInitialized()(test.state()))
			} else {
				require.NoError(t, CheckStateIsInitialized()(test.state()))
			}
		})
	}
}

func TestCheckStateIsUnsealed(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		shouldFail bool
	}{
		"is-unsealed": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = false
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			false,
		},
		"no-state": {
			func() *State { return nil },
			true,
		},
		"no-status": {
			func() *State {
				s := NewState()
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			true,
		},
		"no-health": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false

				return s
			},
			true,
		},
		"no-seal-status": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			true,
		},
		"status-code-and-body-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			true,
		},
		"health-status-code-and-body-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			true,
		},
		"status-and-seal-status-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = false
				s.Health.HealthStatus = HealthStatusInitializedUnsealedActive

				return s
			},
			true,
		},
		"status-and-health-status-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.shouldFail {
				require.Error(t, CheckStateIsUnsealed()(test.state()))
			} else {
				require.NoError(t, CheckStateIsUnsealed()(test.state()))
			}
		})
	}
}

func TestCheckStateIsSealed(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		shouldFail bool
	}{
		"is-sealed": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			false,
		},
		"no-state": {
			func() *State { return nil },
			true,
		},
		"no-status": {
			func() *State {
				s := NewState()
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"no-health": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true

				return s
			},
			true,
		},
		"no-seal-status": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"status-code-and-body-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"health-status-code-and-body-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = false
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"status-and-seal-status-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"status-and-health-status-disagree": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusInitializedUnsealed
				s.Status.Sealed = false
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.shouldFail {
				require.Error(t, CheckStateIsSealed()(test.state()))
			} else {
				require.NoError(t, CheckStateIsSealed()(test.state()))
			}
		})
	}
}

func TestCheckStateSealStateIsKnown(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		shouldFail bool
	}{
		"is-known": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			false,
		},
		"no-state": {
			func() *State { return nil },
			true,
		},
		"no-status": {
			func() *State {
				s := NewState()
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = false
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"status-unknown": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusUnknown
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"no-health": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true

				return s
			},
			true,
		},
		"health-status-unknown": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusUnknown

				return s
			},
			true,
		},
		"no-seal-status": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
		"no-seal-status-data": {
			func() *State {
				s := NewState()
				s.Status = NewStatusResponse()
				s.Status.StatusCode = StatusSealed
				s.Status.Sealed = true
				s.SealStatus = NewSealStatusResponse()
				s.SealStatus.Data = nil
				s.Health = NewHealthResponse()
				s.Health.Sealed = true
				s.Health.HealthStatus = HealthStatusSealed

				return s
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.shouldFail {
				require.Error(t, CheckStateSealStateIsKnown()(test.state()))
			} else {
				require.NoError(t, CheckStateSealStateIsKnown()(test.state()))
			}
		})
	}
}

func TestCheckStateHasHealthStatusOf(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		has        HealthStatus
		wants      HealthStatus
		shouldFail bool
	}{
		"is-initialized-unsealed": {
			HealthStatusInitializedUnsealedActive,
			HealthStatusInitializedUnsealedActive,
			false,
		},
		"not-initialized-unsealed": {
			HealthStatusUnknown,
			HealthStatusInitializedUnsealedActive,
			true,
		},
		"is-unsealed-standby": {
			HealthStatusUnsealedStandby,
			HealthStatusUnsealedStandby,
			false,
		},
		"not-unsealed-standby": {
			HealthStatusNotInitialized,
			HealthStatusUnsealedStandby,
			true,
		},
		"is-dr-secondary-active": {
			HealthStatusDRReplicationSecondaryActive,
			HealthStatusDRReplicationSecondaryActive,
			false,
		},
		"not-dr-secondary-active": {
			HealthStatusPerformanceStandby,
			HealthStatusDRReplicationSecondaryActive,
			true,
		},
		"is-perf-standby": {
			HealthStatusPerformanceStandby,
			HealthStatusPerformanceStandby,
			false,
		},
		"not-perf-standby": {
			HealthStatusDRReplicationSecondaryActive,
			HealthStatusPerformanceStandby,
			true,
		},
		"is-not-initialized": {
			HealthStatusNotInitialized,
			HealthStatusNotInitialized,
			false,
		},
		"is-not-initialized-but-is": {
			HealthStatusPerformanceStandby,
			HealthStatusNotInitialized,
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.Health = NewHealthResponse()
			state.Health.HealthStatus = test.has
			if test.shouldFail {
				require.Error(t, CheckStateHasHealthStatusOf(test.wants)(state))
			} else {
				require.NoError(t, CheckStateHasHealthStatusOf(test.wants)(state))
			}
		})
	}
}

func TestCheckStateHasEnableUIInConfig(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-config": {
			func(*testing.T) *State {
				s := NewState()
				return s
			},
			true,
		},
		"has-enable-ui": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))

				return state
			},
			false,
		},
		"no-enable-ui": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))
				state.ConfigSanitized.Data.EnableUI = false

				return state
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasEnableUIInConfig()(state))
			} else {
				require.NoError(t, CheckStateHasEnableUIInConfig()(state))
			}
		})
	}
}

func TestCheckStateHasMatchingListenerInConfig(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		expected   *ConfigListener
		shouldFail bool
	}{
		"no-state": {
			func(*testing.T) *State {
				s := NewState()
				return s
			},
			&ConfigListener{
				Type: "tcp",
				Config: &ConfigListenerConfig{
					Address:    "0.0.0.0:8200",
					TLSDisable: "true",
				},
			},
			true,
		},
		"has-match": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))

				return state
			},
			&ConfigListener{
				Type: "tcp",
				Config: &ConfigListenerConfig{
					Address:    "0.0.0.0:8200",
					TLSDisable: "true",
				},
			},
			false,
		},
		"wrong-type": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))

				return state
			},
			&ConfigListener{
				Type: "unix",
				Config: &ConfigListenerConfig{
					Address:    "0.0.0.0:8200",
					TLSDisable: "true",
				},
			},
			true,
		},
		"wrong-address": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))

				return state
			},
			&ConfigListener{
				Type: "tcp",
				Config: &ConfigListenerConfig{
					Address:    "0.0.0.1:8200",
					TLSDisable: "true",
				},
			},
			true,
		},
		"wrong-tls": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "config-sanitized.json")
				state := NewState()
				state.ConfigSanitized = NewConfigStateSanitizedResponse()
				require.NoError(t, json.Unmarshal(content, &state.ConfigSanitized))

				return state
			},
			&ConfigListener{
				Type: "tcp",
				Config: &ConfigListenerConfig{
					Address:    "0.0.0.0:8200",
					TLSDisable: "false",
				},
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMatchingListenerInConfig(test.expected)(state))
			} else {
				require.NoError(t, CheckStateHasMatchingListenerInConfig(test.expected)(state))
			}
		})
	}
}

func TestCheckStateHasHAActiveNode(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-nodes-in-ha-status": {
			func(*testing.T) *State {
				s := NewState()
				s.HAStatus = NewHAStatusResponse()
				s.HAStatus.Data = nil

				return s
			},
			true,
		},
		"has-active-node": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "ha-status.json")
				state := NewState()
				state.HAStatus = NewHAStatusResponse()
				require.NoError(t, json.Unmarshal(content, &state.HAStatus))

				return state
			},
			false,
		},
		"no-active-node": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "ha-status.json")
				state := NewState()
				state.HAStatus = NewHAStatusResponse()
				require.NoError(t, json.Unmarshal(content, &state.HAStatus))
				state.HAStatus.Data.Nodes[2].ActiveNode = false

				return state
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasHAActiveNode()(state))
			} else {
				require.NoError(t, CheckStateHasHAActiveNode()(state))
			}
		})
	}
}

func TestCheckStateHasMinHANodes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-ha-status-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "ha-status.json")
				state := NewState()
				state.HAStatus = NewHAStatusResponse()
				require.NoError(t, json.Unmarshal(content, &state.HAStatus))
				cpy := *state.HAStatus.Data.Nodes[0]
				state.HAStatus.Data.Nodes = append(state.HAStatus.Data.Nodes, &cpy)

				return state
			},
			false,
		},
		"not-enough-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "ha-status.json")
				state := NewState()
				state.HAStatus = NewHAStatusResponse()
				require.NoError(t, json.Unmarshal(content, &state.HAStatus))
				state.HAStatus.Data.Nodes = state.HAStatus.Data.Nodes[1:]

				return state
			},
			true,
		},
		"exactly-n-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "ha-status.json")
				state := NewState()
				state.HAStatus = NewHAStatusResponse()
				require.NoError(t, json.Unmarshal(content, &state.HAStatus))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNHANodes(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNHANodes(3)(state))
			}
		})
	}
}

func TestCheckStateHasMinRaftServers(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-servers": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state := NewState()
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))
				cpy := *state.RaftConfig.Data.Config.Servers[0]
				state.RaftConfig.Data.Config.Servers = append(state.RaftConfig.Data.Config.Servers, &cpy)

				return state
			},
			false,
		},
		"not-enough-servers": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))
				state.RaftConfig.Data.Config.Servers = state.RaftConfig.Data.Config.Servers[1:]

				return state
			},
			true,
		},
		"exactly-n-servers": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNRaftServers(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNRaftServers(3)(state))
			}
		})
	}
}

func TestCheckStateHasMinRaftVoters(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state := NewState()
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))
				cpy := *state.RaftConfig.Data.Config.Servers[0]
				state.RaftConfig.Data.Config.Servers = append(state.RaftConfig.Data.Config.Servers, &cpy)

				return state
			},
			false,
		},
		"not-enough-voters": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))
				state.RaftConfig.Data.Config.Servers[0].Voter = false

				return state
			},
			true,
		},
		"exactly-n-voters": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNRaftVoters(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNRaftVoters(3)(state))
			}
		})
	}
}

func TestCheckStateHasRaftLeader(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"has-leader": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state := NewState()
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))

				return state
			},
			false,
		},
		"no-leader": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-configuration.json")
				state.RaftConfig = NewRaftConfigurationResponse()
				require.NoError(t, json.Unmarshal(content, &state.RaftConfig))
				state.RaftConfig.Data.Config.Servers[0].Leader = false

				return state
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasRaftLeader()(state))
			} else {
				require.NoError(t, CheckStateHasRaftLeader()(state))
			}
		})
	}
}

func TestCheckStateHasMinAutopilotServers(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-servers": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				cpy := *state.AutopilotState.Data.Servers["node_0"]
				cpy.Name = "node_4"
				state.AutopilotState.Data.Servers["node_4"] = &cpy

				return state
			},
			false,
		},
		"not-enough-servers": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				delete(state.AutopilotState.Data.Servers, "node_2")

				return state
			},
			true,
		},
		"exactly-n-servers": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNAutopilotServers(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNAutopilotServers(3)(state))
			}
		})
	}
}

func TestCheckStateHasMinAutopilotVoters(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				state.AutopilotState.Data.Voters = append(state.AutopilotState.Data.Voters, "node_4")

				return state
			},
			false,
		},
		"not-enough-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				state.AutopilotState.Data.Voters = state.AutopilotState.Data.Voters[:1]

				return state
			},
			true,
		},
		"exactly-n-voters": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNAutopilotVoters(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNAutopilotVoters(3)(state))
			}
		})
	}
}

func TestCheckStateHasMinAutopilotHealthyNodes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-healthy-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				cpy := *state.AutopilotState.Data.Servers["node_0"]
				cpy.Name = "node_4"
				state.AutopilotState.Data.Servers["node_4"] = &cpy

				return state
			},
			false,
		},
		"not-enough-healthy-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				state.AutopilotState.Data.Servers["node_2"].Healthy = false

				return state
			},
			true,
		},
		"exactly-n-healthy-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))

				return state
			},
			false,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateHasMinNAutopilotHealthyNodes(3)(state))
			} else {
				require.NoError(t, CheckStateHasMinNAutopilotHealthyNodes(3)(state))
			}
		})
	}
}

func TestCheckStateAutopilotIsHealthy(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func() *State { return NewState() },
			true,
		},
		"healthy": {
			func() *State {
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				state.AutopilotState.Data.Healthy = true

				return state
			},
			false,
		},
		"not-healthy": {
			func() *State {
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				state.AutopilotState.Data.Healthy = false

				return state
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state()
			if test.shouldFail {
				require.Error(t, CheckStateAutopilotIsHealthy()(state))
			} else {
				require.NoError(t, CheckStateAutopilotIsHealthy()(state))
			}
		})
	}
}

func TestCheckStateAutopilotHasLeader(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-raft-config-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"has-leader": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state := NewState()
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))

				return state
			},
			false,
		},
		"no-leader": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				state.AutopilotState.Data.Leader = ""
				state.AutopilotState.Data.Servers["node_0"].Status = "voter"

				return state
			},
			true,
		},
		"mismatch-leader": {
			func(t *testing.T) *State {
				t.Helper()
				state := NewState()
				content := testReadSupport(t, "storage-raft-autopilot-state.json")
				state.AutopilotState = NewRaftAutopilotStateResponse()
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				require.NoError(t, json.Unmarshal(content, &state.AutopilotState))
				state.AutopilotState.Data.Leader = "node_1"

				return state
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateAutopilotHasLeader()(state))
			} else {
				require.NoError(t, CheckStateAutopilotHasLeader()(state))
			}
		})
	}
}

func TestCheckStateHasStorageType(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		expected   string
		shouldFail bool
	}{
		"no-seal-status-data": {
			func() *State { return NewState() },
			"raft",
			true,
		},
		"has-match": {
			func() *State {
				t.Helper()
				state := NewState()
				state.SealStatus = NewSealStatusResponse()
				state.SealStatus.Data = &SealStatusResponseData{}
				state.SealStatus.Data.StorageType = "raft"

				return state
			},
			"raft",
			false,
		},
		"no-match": {
			func() *State {
				t.Helper()
				state := NewState()
				state.SealStatus = NewSealStatusResponse()
				state.SealStatus.Data = &SealStatusResponseData{}
				state.SealStatus.Data.StorageType = "consul"

				return state
			},
			"raft",
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state()
			if test.shouldFail {
				require.Error(t, CheckStateHasStorageType("raft")(state))
			} else {
				require.NoError(t, CheckStateHasStorageType("raft")(state))
			}
		})
	}
}

func TestCheckStateHasSealType(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func() *State
		expected   string
		shouldFail bool
	}{
		"no-seal-status-data": {
			func() *State { return NewState() },
			"shamir",
			true,
		},
		"has-match": {
			func() *State {
				t.Helper()
				state := NewState()
				state.SealStatus = NewSealStatusResponse()
				state.SealStatus.Data = &SealStatusResponseData{}
				state.SealStatus.Data.Type = "shamir"

				return state
			},
			"shamir",
			false,
		},
		"no-match": {
			func() *State {
				t.Helper()
				state := NewState()
				state.SealStatus = NewSealStatusResponse()
				state.SealStatus.Data = &SealStatusResponseData{}
				state.SealStatus.Data.Type = "awskms"

				return state
			},
			"shamir",
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state()
			if test.shouldFail {
				require.Error(t, CheckStateHasSealType("shamir")(state))
			} else {
				require.NoError(t, CheckStateHasSealType("shamir")(state))
			}
		})
	}
}

func testReadSupport(t *testing.T, name string) []byte {
	t.Helper()

	p, err := filepath.Abs(filepath.Join("./support", name))
	require.NoError(t, err)
	f, err := os.Open(p)
	require.NoError(t, err)
	content, err := io.ReadAll(f)
	require.NoError(t, err)

	return content
}
