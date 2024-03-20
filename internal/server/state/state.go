// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type Serializable interface {
	Terraform5Type() tftypes.Type
	Terraform5Value() tftypes.Value
	FromTerraform5Value(val tftypes.Value) error
}

type State interface {
	Serializable

	Schema() *tfprotov6.Schema
	Validate(ctx context.Context) error

	// HandleFailure is called when either an apply or plan fails, diag is the tfprotov6.Diagnostic
	// for the failure, providerConfig is the tftypes.Value of the provider configuration.
	HandleFailure(ctx context.Context, diag *tfprotov6.Diagnostic, providerConfig tftypes.Value)
}

// Marshal converts a Serializable state value into a DynamicValue suitable for transporting over the
// wire in response to the various Terraform callbacks, i.e. PlanResourceChange or ApplyResourceChange
// The generated value must have the structure as the value received in the request from Terraform,
// otherwise Terraform will blow up with an error.
func Marshal(serializable Serializable) (*tfprotov6.DynamicValue, error) {
	dyn, err := tfprotov6.NewDynamicValue(serializable.Terraform5Type(), serializable.Terraform5Value())
	if err != nil {
		return &dyn, fmt.Errorf("failed to marshal the state, due to: %w", err)
	}

	return &dyn, nil
}

// MarshalDelete creates a nil Terraform DynamicValue, that indicates that the resource has been deleted.
func MarshalDelete(serializable Serializable) (*tfprotov6.DynamicValue, error) {
	dyn, err := tfprotov6.NewDynamicValue(serializable.Terraform5Type(), tftypes.NewValue(serializable.Terraform5Type(), nil))
	if err != nil {
		err = fmt.Errorf("failed to marshal the state, due to: %w", err)
	}

	return &dyn, err
}
