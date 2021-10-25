package plugin

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type State interface {
	Schema() *tfprotov6.Schema
	Validate(context.Context) error
	Serializable
}

type Serializable interface {
	Terraform5Type() tftypes.Type
	Terraform5Value() tftypes.Value
	FromTerraform5Value(val tftypes.Value) error
}

type TFType interface {
	TFType() tftypes.Type
	TFValue() tftypes.Value
	FromTFValue(val tftypes.Value) error
}

type StateWithTransport interface {
	State
	EmbeddedTransport() *embeddedTransportV1
}

type ResourceWithProviderConfig interface {
	SetProviderConfig(tftypes.Value) error
	GetProviderConfig() (*config, error)
}
