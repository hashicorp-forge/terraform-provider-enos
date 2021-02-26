package plugin

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type transport struct {
	state *dataTransportStateV1
}

var _ datarouter.DataSource = (*transport)(nil)

type dataTransportStateV1 struct {
	ID        string
	Transport *embeddedTransportV1
}

var _ State = (*dataTransportStateV1)(nil)

func newTransport() *transport {
	return &transport{
		state: newDataTransportState(),
	}
}

func newDataTransportState() *dataTransportStateV1 {
	return &dataTransportStateV1{
		Transport: newEmbeddedTransport(),
	}
}

func (t *transport) Name() string {
	return "enos_transport"
}

func (t *transport) Schema() *tfprotov5.Schema {
	return t.state.Schema()
}

// ValidateDataSourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (t *transport) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	res := &tfprotov5.ValidateDataSourceConfigResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	// unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	newConfig := newDataTransportState()
	err := unmarshal(newConfig, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}

	return res, err
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (t *transport) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	res := &tfprotov5.ReadDataSourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	// unmarshal and re-marshal the state to add default fields
	err := unmarshal(t.state, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.State, err = marshal(t.state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, nil
}

// Schema is the data source's schema.
func (ts *dataTransportStateV1) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				ts.Transport.SchemaAttributeOut(),
			},
			BlockTypes: []*tfprotov5.SchemaNestedBlock{
				{
					TypeName: "ssh",
					Nesting:  tfprotov5.SchemaNestedBlockNestingModeSingle,
					Block: &tfprotov5.SchemaBlock{
						Attributes: ts.SchemaAttributesSSH(),
					},
				},
			},
		},
	}
}

// SchemaAttributesSSH is the data source's SSH schema.
func (ts *dataTransportStateV1) SchemaAttributesSSH() []*tfprotov5.SchemaAttribute {
	return []*tfprotov5.SchemaAttribute{
		{
			Name:     "user",
			Type:     tftypes.String,
			Optional: true,
		},
		{
			Name:     "host",
			Type:     tftypes.String,
			Optional: true,
		},
		{
			Name:     "private_key",
			Type:     tftypes.String,
			Optional: true,
		},
		{
			Name:     "private_key_path",
			Type:     tftypes.String,
			Optional: true,
		},
		{
			Name:     "passphrase",
			Type:     tftypes.String,
			Optional: true,
		},
		{
			Name:     "passphrase_path",
			Type:     tftypes.String,
			Optional: true,
		},
	}
}

// FromTerraform5ValueSSH is a callback to unmarshal the SSH block
func (ts *dataTransportStateV1) FromTerraform5ValueSSH(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"user":             &ts.Transport.SSH.User,
		"host":             &ts.Transport.SSH.Host,
		"private_key":      &ts.Transport.SSH.PrivateKey,
		"private_key_path": &ts.Transport.SSH.PrivateKeyPath,
		"passphrase":       &ts.Transport.SSH.Passphrase,
		"passphrase_path":  &ts.Transport.SSH.PassphrasePath,
	})

	return err
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Value with As().
func (ts *dataTransportStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id": &ts.ID,
	})
	if err != nil {
		return err
	}

	if vals["ssh"].IsNull() {
		return nil
	}

	return ts.FromTerraform5ValueSSH(vals["ssh"])
}

// We don't really need to validate at this point as the data source is essentially
// a glorified complex variable and it can be used in combination with resource
// defined transport settings. As such, we'll defer proper validation to the
// embedded transport in the resource.
func (ts *dataTransportStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Terraform5Type is the tftypes.Type for the data transport state.
func (ts *dataTransportStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"id":  tftypes.String,
			"out": ts.Transport.Terraform5Type(),
			"ssh": ts.Terraform5TypeSSH(),
		},
	}
}

// Terraform5Type is the tftypes.Value for the data transport state.
func (ts *dataTransportStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(ts.Terraform5Type(), map[string]tftypes.Value{
		"id":  tftypes.NewValue(tftypes.String, "static"),
		"out": ts.Transport.Terraform5Value(),
		"ssh": ts.Terraform5ValueSSH(),
	})
}

// Terraform5TypeSSH is the tftypes.Type for the data transport SSH state.
func (ts *dataTransportStateV1) Terraform5TypeSSH() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"user":             tftypes.String,
			"host":             tftypes.String,
			"private_key":      tftypes.String,
			"private_key_path": tftypes.String,
			"passphrase":       tftypes.String,
			"passphrase_path":  tftypes.String,
		},
	}
}

// Terraform5ValueSSH is the tftypes.Value for the data transport SSH state.
func (ts *dataTransportStateV1) Terraform5ValueSSH() tftypes.Value {
	return tftypes.NewValue(ts.Terraform5TypeSSH(), map[string]tftypes.Value{
		"user":             tfMarshalStringValue(ts.Transport.SSH.User),
		"host":             tfMarshalStringValue(ts.Transport.SSH.Host),
		"private_key":      tfMarshalStringValue(ts.Transport.SSH.PrivateKey),
		"private_key_path": tfMarshalStringValue(ts.Transport.SSH.PrivateKeyPath),
		"passphrase":       tfMarshalStringValue(ts.Transport.SSH.Passphrase),
		"passphrase_path":  tfMarshalStringValue(ts.Transport.SSH.PassphrasePath),
	})
}
