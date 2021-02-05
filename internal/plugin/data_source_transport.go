package plugin

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type transport struct {
	SSHUser       string
	SSHHost       string
	SSHPrivateKey string
}

var _ datarouter.DataSource = (*transport)(nil)

func (t *transport) Name() string {
	return "enos_transport"
}

func (t *transport) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
			},
			BlockTypes: []*tfprotov5.SchemaNestedBlock{
				{
					TypeName: "ssh",
					Nesting:  tfprotov5.SchemaNestedBlockNestingModeSingle,
					Block: &tfprotov5.SchemaBlock{
						Attributes: []*tfprotov5.SchemaAttribute{
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
						},
					},
				},
			},
		},
	}
}

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

	// Unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	err := t.Unmarshal(req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}

	return res, err
}

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

	// Unmarshal and re-marshal the state to add default fields
	err := t.Unmarshal(req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	state, err := t.Marshal()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	res.State = &state

	return res, nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (t *transport) FromTerraform5Value(val tftypes.Value) error {
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}

	if vals["ssh"].IsNull() {
		return nil
	}

	ssh := map[string]tftypes.Value{}
	err = vals["ssh"].As(&ssh)
	if err != nil {
		return err
	}

	if ssh["user"].IsKnown() && !ssh["user"].IsNull() {
		err = ssh["user"].As(&t.SSHUser)
		if err != nil {
			return err
		}
	}

	if ssh["host"].IsKnown() && !ssh["host"].IsNull() {
		err = ssh["host"].As(&t.SSHHost)
		if err != nil {
			return err
		}
	}

	if ssh["private_key"].IsKnown() {
		err = ssh["private_key"].As(&t.SSHPrivateKey)
		if err != nil {
			return err
		}
	}

	return err
}

// Unmarshal from a a DynamicValue request
func (t *transport) Unmarshal(req *tfprotov5.DynamicValue) error {
	tfType, err := dynToValue(req, t.tfType())
	if err != nil {
		return err
	}

	return tfType.As(t)
}

// Marshal the object state into the proto5 DynamicValue format
func (t *transport) Marshal() (tfprotov5.DynamicValue, error) {
	return tfprotov5.NewDynamicValue(t.tfType(), t.tfValue())
}

func (t *transport) tfType() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"ssh": t.tfTypeSSH(),
			"id":  tftypes.String,
		},
	}
}

func (t *transport) tfTypeSSH() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"user":        tftypes.String,
			"host":        tftypes.String,
			"private_key": tftypes.String,
		},
	}
}

func (t *transport) tfValueSSH() tftypes.Value {
	return tftypes.NewValue(t.tfTypeSSH(), map[string]tftypes.Value{
		"user":        tftypes.NewValue(tftypes.String, t.SSHUser),
		"host":        tftypes.NewValue(tftypes.String, t.SSHHost),
		"private_key": tftypes.NewValue(tftypes.String, t.SSHPrivateKey),
	})
}

func (t *transport) tfValue() tftypes.Value {
	return tftypes.NewValue(t.tfType(), map[string]tftypes.Value{
		"id":  tftypes.NewValue(tftypes.String, "static"),
		"ssh": t.tfValueSSH(),
	})
}
