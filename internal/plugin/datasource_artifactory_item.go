package plugin

import (
	"context"
	"fmt"
	"path"

	"github.com/asaskevich/govalidator"

	"github.com/hashicorp/enos-provider/internal/diags"

	"github.com/hashicorp/enos-provider/internal/server/state"

	"github.com/hashicorp/enos-provider/internal/artifactory"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type artifactoryItem struct {
	providerConfig *config
}

var _ datarouter.DataSource = (*artifactoryItem)(nil)

type artifactoryItemStateV1 struct {
	ID         *tfString
	Username   *tfString
	Token      *tfString
	Host       *tfString
	Repo       *tfString
	Path       *tfString
	Name       *tfString
	Properties *tfStringMap
	Results    *tfObjectSlice
}

var _ state.State = (*artifactoryItemStateV1)(nil)

func newArtifactoryItem() *artifactoryItem {
	return &artifactoryItem{
		providerConfig: newProviderConfig(),
	}
}

func newArtifactoryItemStateV1() *artifactoryItemStateV1 {
	results := newTfObjectSlice()
	results.AttrTypes = map[string]tftypes.Type{
		"name":   tftypes.String,
		"type":   tftypes.String,
		"url":    tftypes.String,
		"sha256": tftypes.String,
		"size":   tftypes.String,
	}

	return &artifactoryItemStateV1{
		ID:         newTfString(),
		Username:   newTfString(),
		Token:      newTfString(),
		Host:       newTfString(),
		Repo:       newTfString(),
		Path:       newTfString(),
		Name:       newTfString(),
		Properties: newTfStringMap(),
		Results:    results,
	}
}

func (d *artifactoryItem) Name() string {
	return "enos_artifactory_item"
}

func (d *artifactoryItem) Schema() *tfprotov6.Schema {
	return newArtifactoryItemStateV1().Schema()
}

func (d *artifactoryItem) SetProviderConfig(meta tftypes.Value) error {
	return d.providerConfig.FromTerraform5Value(meta)
}

// ValidateDataResourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (d *artifactoryItem) ValidateDataResourceConfig(ctx context.Context, req tfprotov6.ValidateDataResourceConfigRequest, res *tfprotov6.ValidateDataResourceConfigResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	// unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	newConfig := newArtifactoryItemStateV1()
	if err := unmarshal(newConfig, req.Config); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	if err := newConfig.Validate(ctx); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Failure", err))
		return
	}
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (d *artifactoryItem) ReadDataSource(ctx context.Context, req tfprotov6.ReadDataSourceRequest, res *tfprotov6.ReadDataSourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	newState := newArtifactoryItemStateV1()

	// unmarshal and re-marshal the state to add default fields
	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	newState.ID.Set("static")

	if err := newState.Validate(ctx); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Failure", err))
		return
	}

	err = newState.Search(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Search Failed", err))
		return
	}

	res.State, err = state.Marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Failed to Marshal", err))
		return
	}
}

// Schema is the file states Terraform schema.
func (s *artifactoryItemStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
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
					Type:     tftypes.Map{ElementType: tftypes.String},
					Optional: true,
				},
				{
					Name:     "results",
					Type:     s.Results.TFType(),
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

	if !s.Host.Unknown {
		host, ok := s.Host.Get()
		if !ok || !govalidator.IsURL(host) {
			return ValidationError("the host must be a valid URL", "host")
		}
	}
	return nil
}

// FromTerraform5Value is a callback to unmarshal from the		tftypes.Vault with As().
func (s *artifactoryItemStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":         s.ID,
		"username":   s.Username,
		"token":      s.Token,
		"host":       s.Host,
		"repo":       s.Repo,
		"path":       s.Path,
		"name":       s.Name,
		"properties": s.Properties,
		"results":    s.Results,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *artifactoryItemStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":         s.ID.TFType(),
		"username":   s.Username.TFType(),
		"token":      s.Token.TFType(),
		"host":       s.Host.TFType(),
		"repo":       s.Repo.TFType(),
		"path":       s.Path.TFType(),
		"name":       s.Name.TFType(),
		"properties": s.Properties.TFType(),
		"results":    s.Results.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *artifactoryItemStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":         s.ID.TFValue(),
		"username":   s.Username.TFValue(),
		"token":      s.Token.TFValue(),
		"host":       s.Host.TFValue(),
		"repo":       s.Repo.TFValue(),
		"path":       s.Path.TFValue(),
		"name":       s.Name.TFValue(),
		"properties": s.Properties.TFValue(),
		"results":    s.Results.TFValue(),
	})
}

// Search queries the aritfactory API and parses the results
func (s *artifactoryItemStateV1) Search(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	host := s.Host.Value()
	client := artifactory.NewClient(
		artifactory.WithHost(host),
		artifactory.WithUsername(s.Username.Value()),
		artifactory.WithToken(s.Token.Value()),
	)

	reqArgs := []artifactory.SearchAQLOpt{}
	if repo, ok := s.Repo.Get(); ok {
		reqArgs = append(reqArgs, artifactory.WithRepo(repo))
	}
	if path, ok := s.Path.Get(); ok {
		reqArgs = append(reqArgs, artifactory.WithPath(path))
	}
	if name, ok := s.Name.Get(); ok {
		reqArgs = append(reqArgs, artifactory.WithName(name))
	}
	if props, ok := s.Properties.GetStrings(); ok {
		reqArgs = append(reqArgs, artifactory.WithProperties(props))
	}

	req := artifactory.NewSearchAQLRequest(reqArgs...)
	res, err := client.SearchAQL(ctx, req)
	if err != nil {
		return fmt.Errorf("search failed, due to: %w", err)
	}

	results := []*tfObject{}
	for _, result := range res.Results {
		r := newTfObject()
		resName := newTfString()
		resName.Set(result.Name)
		resType := newTfString()
		resType.Set(result.Type)
		resURL := newTfString()
		resURL.Set(fmt.Sprintf("%s/%s", host, path.Join(result.Repo, result.Path, result.Name)))
		resSHA256 := newTfString()
		resSHA256.Set(result.SHA256)
		resSize := newTfString()
		resSize.Set(result.Size.String())

		r.Set(map[string]interface{}{
			"name":   resName,
			"type":   resType,
			"url":    resURL,
			"sha256": resSHA256,
			"size":   resSize,
		})
		results = append(results, r)
	}

	s.Results.Set(results)

	return nil
}

func (s *artifactoryItemStateV1) Debug() string {
	return ""
}
