// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
