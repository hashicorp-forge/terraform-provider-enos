// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/k8s"
)

// UnsealRequest is a Vault unseal request.
type UnsealRequest struct {
	*StateRequest
	StateRequestOpts []StateRequestOpt
	*UnsealArguments
}

// UnsealResponse is a Vault unseal response.
type UnsealResponse struct {
	PriorState *State
	PostState  *State
}

type UnsealArguments struct {
	SealType   SealType `json:"seal_type"`
	UnsealKeys []string `json:"unseal_keys"`
}

// SealType is the Vault seal type.
type SealType string

// SealTypes are the possible Vault seal types.
const (
	SealTypeShamir        SealType = "shamir"
	SealTypeAliCloud      SealType = "alicloudkms"
	SealTypeAWSKMS        SealType = "awskms"
	SealTypeAzureKeyVault SealType = "azurekeyvault"
	SealTypeGCPKMS        SealType = "gcpkms"
	SealTypeOCIKMS        SealType = "ocikms"
	SealTypeHSMPKCS11     SealType = "pkcs11"
	SealTypeTransit       SealType = "transit"
)

// UnsealRequestOpt is a functional option for a unseal request.
type UnsealRequestOpt func(*UnsealRequest) *UnsealRequest

// NewUnsealRequest takes functional options and returns a new
// unseal request.
func NewUnsealRequest(opts ...UnsealRequestOpt) *UnsealRequest {
	c := &UnsealRequest{
		StateRequest: NewStateRequest(),
		UnsealArguments: &UnsealArguments{
			SealType:   "",
			UnsealKeys: []string{},
		},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	for _, opt := range c.StateRequestOpts {
		opt(c.StateRequest)
	}

	return c
}

// WithUnsealStateRequestOpts sets the state request options.
func WithUnsealStateRequestOpts(opts ...StateRequestOpt) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.StateRequestOpts = opts
		return u
	}
}

// WithUnsealRequestSealType sets the Vault seal type.
func WithUnsealRequestSealType(typ SealType) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.SealType = typ
		return u
	}
}

// WithUnsealRequestUnsealKeys sets the Vault unseal keys.
func WithUnsealRequestUnsealKeys(unsealKeys []string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.UnsealKeys = unsealKeys
		return u
	}
}

// Unseal checks the current steal status, and if needed unseals the Vault in
// different ways depending on seal type.
func Unseal(ctx context.Context, tr it.Transport, req *UnsealRequest) (*UnsealResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	binPath := req.BinPath
	if binPath == "" {
		return nil, errors.New("you must supply a vault bin path")
	}

	vaultAddr := req.VaultAddr
	if vaultAddr == "" {
		return nil, errors.New("you must supply a vault listen address")
	}

	var err error
	res := &UnsealResponse{}

	priorStateChecks := []CheckStater{
		CheckStateIsInitialized(),
		CheckStateSealStateIsKnown(),
	}
	postStateChecks := []CheckStater{
		CheckStateIsInitialized(),
		CheckStateIsUnsealed(),
	}

	// BUG(vault_status): Only enforce the seal type check for shamir as the seal-status API
	// is broken when using auto-unseal methods. When the issue is resolved we can assert it here.
	// If vault_status is implemented before the bug is fixed we should assert the seal-type
	// separately and output a warning diagnostic.
	//
	// Further reading:
	// - https://hashicorp.atlassian.net/browse/VAULT-7061
	if req.SealType == SealTypeShamir {
		postStateChecks = append(postStateChecks, CheckStateHasSealType(req.SealType))
	}

	switch tr.Type() {
	case it.TransportType("ssh"):
		priorStateChecks = append(priorStateChecks, CheckStateHasSystemdEnabledAndRunningProperties())
		postStateChecks = append(postStateChecks, CheckStateHasSystemdEnabledAndRunningProperties())
	case it.TransportType("kubernetes"):
		k, ok := tr.(*k8s.Transport)
		if ok {
			priorStateChecks = append(priorStateChecks, CheckStatePodHasPhase(k.Pod, v1.PodRunning))
			postStateChecks = append(postStateChecks, CheckStatePodHasPhase(k.Pod, v1.PodRunning))
		}
	default:
	}

	// Wait for vault to ready to unseal.
	res.PriorState, err = WaitForState(ctx, tr, req.StateRequest, priorStateChecks...)
	if err != nil {
		return res, fmt.Errorf("waiting for vault cluster to be ready to unseal: %w", err)
	}

	sealed, err := res.PriorState.IsSealed()
	if err != nil {
		return res, fmt.Errorf("checking vault seal status before unseal: %w", err)
	}

	getPostState := func(res *UnsealResponse) error {
		var err error
		res.PostState, err = GetState(ctx, tr, req.StateRequest)

		return err
	}

	if sealed {
		switch req.SealType {
		case SealTypeShamir:
			for i, key := range req.UnsealKeys {
				select {
				case <-ctx.Done():
					err = errors.Join(
						ctx.Err(),
						fmt.Errorf("timed out when unsealing with key: (%d) %s", i, key),
						getPostState(res),
					)

					return res, err
				default:
				}

				_, stderr, err := tr.Run(ctx, command.New(
					fmt.Sprintf("%s operator unseal %s;", binPath, key),
					command.WithEnvVar("VAULT_ADDR", vaultAddr)),
				)
				if err != nil {
					err = errors.Join(
						err,
						fmt.Errorf("failed unsealing with key: (%d) %s, stderr: \n%s", i, key, stderr),
						getPostState(res),
					)

					return res, err
				}
			}
		case SealTypeAliCloud, SealTypeAWSKMS, SealTypeAzureKeyVault, SealTypeGCPKMS,
			SealTypeOCIKMS, SealTypeHSMPKCS11, SealTypeTransit:
			// Let auto-unseal take the wheel and wait for vault to be unsealed below.
		default:
			return res, fmt.Errorf("unspported seal type: %s", req.SealType)
		}
	}

	// Wait for vault to be unsealed.
	res.PostState, err = WaitForState(ctx, tr, req.StateRequest, postStateChecks...)
	if err != nil {
		return res, fmt.Errorf("waiting for vault cluster to be ready to be unsealed: %w", err)
	}

	return res, nil
}
