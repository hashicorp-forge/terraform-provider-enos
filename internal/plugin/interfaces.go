// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
)

type TFType interface {
	fmt.Stringer
	TFType() tftypes.Type
	TFValue() tftypes.Value
	FromTFValue(val tftypes.Value) error
}

type StateWithTransport interface {
	state.State
	EmbeddedTransport() *embeddedTransportV1
}

type ResourceWithProviderConfig interface {
	SetProviderConfig(val tftypes.Value) error
	GetProviderConfig() (*config, error)
}
