package plugin

import (
	"context"
	"fmt"
	"path"

	"github.com/asaskevich/govalidator"

	"github.com/hashicorp/enos-provider/internal/artifactory"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type artifactoryItem struct {
	providerConfig *config
}

var _ datarouter.DataSource = (*artifactoryItem)(nil)

type artifactoryItemStateV1 struct {
	ID         string
	Username   string
	Token      string
	Host       string
	Repo       string
	Path       string
	Name       string
	Properties map[string]string
	Results    []map[string]string
}

var _ State = (*artifactoryItemStateV1)(nil)

func newArtifactoryItem() *artifactoryItem {
	return &artifactoryItem{
		providerConfig: newProviderConfig(),
	}
}

func newArtifactoryItemStateV1() *artifactoryItemStateV1 {
	return &artifactoryItemStateV1{
		Properties: map[string]string{},
		Results:    []map[string]string{},
	}
}

func (d *artifactoryItem) Name() string {
	return "enos_artifactory_item"
}

func (d *artifactoryItem) Schema() *tfprotov5.Schema {
	return newArtifactoryItemStateV1().Schema()
}

func (d *artifactoryItem) SetProviderConfig(meta tftypes.Value) error {
	return d.providerConfig.FromTerraform5Value(meta)
}

// ValidateDataSourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (d *artifactoryItem) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
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
	newConfig := newArtifactoryItemStateV1()
	err := unmarshal(newConfig, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}

	return res, err
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (d *artifactoryItem) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	res := &tfprotov5.ReadDataSourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	newState := newArtifactoryItemStateV1()

	// unmarshal and re-marshal the state to add default fields
	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	newState.ID = "static"

	err = newState.Search(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.State, err = marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, nil
}

// Schema is the file states Terraform schema.
func (s *artifactoryItemStateV1) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "username",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:      "token",
					Type:      tftypes.String,
					Required:  true,
					Sensitive: true,
				},
				{
					Name:     "host",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "repo",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "path",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "name",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "properties",
					Type:     tftypes.Map{AttributeType: tftypes.String},
					Optional: true,
				},
				{
					Name:     "results",
					Type:     s.ResultsTerraform5Type(),
					Computed: true,
				},
			},
		},
	}
}

// Validate validates the configuration.
func (s *artifactoryItemStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !govalidator.IsURL(s.Host) {
		return newErrWithDiagnostics("invalid configuration", "the host must be a valid URL", "host")
	}

	if !govalidator.IsEmail(s.Username) {
		return newErrWithDiagnostics("invalid configuration", "the username must be a valid email address", "username")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the		tftypes.Vault with As().
func (s *artifactoryItemStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":       &s.ID,
		"username": &s.Username,
		"token":    &s.Token,
		"host":     &s.Host,
		"repo":     &s.Repo,
		"path":     &s.Path,
		"name":     &s.Name,
	})
	if err != nil {
		return err
	}

	props, ok := vals["properties"]
	if ok {
		if props.IsKnown() && !props.IsNull() {
			s.Properties, err = tfUnmarshalStringMap(props)
			if err != nil {
				return err
			}
		}
	}

	results, ok := vals["results"]
	if ok {
		if results.IsKnown() && !results.IsNull() {
			// Get a list of all the results as values
			resVals := []tftypes.Value{}
			err := results.As(&resVals)
			if err != nil {
				return err
			}

			// Convert the result values into our results
			for _, res := range resVals {
				if res.IsKnown() && !res.IsNull() {
					to := map[string]string{}
					err := res.As(&to)
					if err != nil {
						return err
					}

					s.Results = append(s.Results, to)
				}
			}
		}
	}

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *artifactoryItemStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":         tftypes.String,
		"username":   tftypes.String,
		"token":      tftypes.String,
		"host":       tftypes.String,
		"repo":       tftypes.String,
		"path":       tftypes.String,
		"name":       tftypes.String,
		"properties": tftypes.Map{AttributeType: tftypes.String},
		"results":    s.ResultsTerraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *artifactoryItemStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":         tfMarshalStringValue(s.ID),
		"username":   tfMarshalStringValue(s.Username),
		"token":      tfMarshalStringValue(s.Token),
		"host":       tfMarshalStringValue(s.Host),
		"repo":       tfMarshalStringOptionalValue(s.Repo),
		"path":       tfMarshalStringOptionalValue(s.Path),
		"name":       tfMarshalStringOptionalValue(s.Name),
		"properties": tfMarshalStringMap(s.Properties),
		"results":    s.ResultsTerraform5Value(),
	})
}

// ResultsTerraform5Type is the results attribute as a terraform type
func (s *artifactoryItemStateV1) ResultsTerraform5Type() tftypes.Type {
	return tftypes.List{ElementType: s.ResultTerraform5Type()}
}

// ResultTerraform5AttributeTypes are a results attributes as a terraform type
func (s *artifactoryItemStateV1) ResultTerraform5AttributeTypes() map[string]tftypes.Type {
	return map[string]tftypes.Type{
		"name":   tftypes.String,
		"type":   tftypes.String,
		"url":    tftypes.String,
		"sha256": tftypes.String,
		"size":   tftypes.String,
	}
}

// ResultTerraform5Type is and individual result as a terraform type
func (s *artifactoryItemStateV1) ResultTerraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: s.ResultTerraform5AttributeTypes(),
	}
}

// ResultsTerraform5Value is the results as a terraform value
func (s *artifactoryItemStateV1) ResultsTerraform5Value() tftypes.Value {
	resVals := []tftypes.Value{}

	for _, res := range s.Results {
		resVal := map[string]tftypes.Value{}

		for attr := range s.ResultTerraform5AttributeTypes() {
			v, ok := res[attr]
			if ok {
				resVal[attr] = tfMarshalStringValue(v)
			}
		}

		resVals = append(resVals, tftypes.NewValue(s.ResultTerraform5Type(), resVal))
	}

	return tftypes.NewValue(s.ResultsTerraform5Type(), resVals)
}

// Search queries the aritfactory API and parses the results
func (s *artifactoryItemStateV1) Search(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	client := artifactory.NewClient(
		artifactory.WithHost(s.Host),
		artifactory.WithUsername(s.Username),
		artifactory.WithToken(s.Token),
	)

	reqArgs := []artifactory.SearchAQLOpt{}
	if s.Repo != "" && s.Repo != UnknownString {
		reqArgs = append(reqArgs, artifactory.WithRepo(s.Repo))
	}
	if s.Path != "" && s.Path != UnknownString {
		reqArgs = append(reqArgs, artifactory.WithPath(s.Path))
	}
	if s.Name != "" && s.Name != UnknownString {
		reqArgs = append(reqArgs, artifactory.WithName(s.Name))
	}
	if len(s.Properties) > 0 {
		reqArgs = append(reqArgs, artifactory.WithProperties(s.Properties))
	}

	req := artifactory.NewSearchAQLRequest(reqArgs...)
	res, err := client.SearchAQL(ctx, req)
	if err != nil {
		return wrapErrWithDiagnostics(err, "search failure", "failed to search for artifactory item")
	}

	for _, result := range res.Results {
		s.Results = append(s.Results, map[string]string{
			"name":   result.Name,
			"type":   result.Type,
			"url":    fmt.Sprintf("%s/%s", s.Host, path.Join(result.Repo, result.Path, result.Name)),
			"sha256": result.SHA256,
			"size":   result.Size.String(),
		})
	}

	return nil
}
