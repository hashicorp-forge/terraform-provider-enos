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
	// setResolvedTransport sets the transport state that was resolved by applying the provider
	// defaults to the embedded transport this state
	setResolvedTransport(transport transportState)
}

type ResourceWithProviderConfig interface {
	SetProviderConfig(tftypes.Value) error
	GetProviderConfig() (*config, error)
}
