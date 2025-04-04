// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"path"
	"text/template"

	"github.com/asaskevich/govalidator"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/artifactory"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/datarouter"
)

type artifactoryItem struct {
	providerConfig *config
}

var _ datarouter.DataSource = (*artifactoryItem)(nil)

type artifactoryItemStateV1 struct {
	ID            *tfString
	Username      *tfString
	Token         *tfString
	Host          *tfString
	Repo          *tfString
	Path          *tfString
	Name          *tfString
	Properties    *tfStringMap
	QueryTemplate *tfString
	Results       *tfObjectSlice

	failureHandlers
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
		ID:              newTfString(),
		Username:        newTfString(),
		Token:           newTfString(),
		Host:            newTfString(),
		Repo:            newTfString(),
		Path:            newTfString(),
		Name:            newTfString(),
		Properties:      newTfStringMap(),
		QueryTemplate:   newTfString(),
		Results:         results,
		failureHandlers: failureHandlers{},
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

	if err = newState.Validate(ctx); err != nil {
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
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_artifactory_item^ datasource is a datasource that you can use to search for items in
artifactory. This is useful for finding build artifact URLs that we can install on targets for testing.
The datasource will return a list of ^results^ which include ^name^s and ^url^s to all matching items.

There are two primary modes for using the datasource for searching. One for a ^properties^ based match
and another where a ^query_template^ is provided.

For the ^properties^ based search, configure the datasource with a ^name^ and list of ^properties^ that
are expected and the datasource will automatically generate a query where and search with. Each property
is included in the ^items.find()^ query with a ^$match^ operator. The more specific your search criteria,
via the ^path^, ^name^, and ^properties^, the fewer results you'll receive.

For the ^query_template^, the ^repo^, ^path^, ^name^, and ^properties^ attributes are not automatically
included in a query for you. Instead, you provide a Go text template which includes the entire query.
This is an advanced option for hand crafted artisinal queries. As it is a Go template, you can provide
pure text string or include [Go text template](https://pkg.go.dev/text/template#Template) directives.
For the latter, you can expect the evaluation context to include an object with the following attributes:
  - Repo          string
  - Path          string
  - Name          string
  - Properties    map[string]string

For example:
^^^hcl
query_template = <<EOQ
items.find({
  "repo":           { "$match": "{{ .Repo }}" },
  "path":           { "$match": "{{ .Path }}" },
  "name":           { "$match": "{{ .Name }}" },
  "stat.downloads": { "$gt": "${var.min_download} "}
}).include("*", "property.*") .sort({"$desc": ["modified"]})
EOQ
^^^

*NOTE*: The underlying implementation uses AQL to search for artifacts and uses the ^$match^ operator
for every criteria. This means that you can use wildcards ^*^ for any field. See the [AQL developer guide](https://www.jfrog.com/confluence/display/JFROG/Artifactory+Query+Language) for more information.
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: datasourceStaticIDDescription,
				},
				{
					Name:        "username",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The Artifactory API Key user name. Depending on your login scheme this is likely an email address. If no username is provided we'll assume you wish to use an identity token for Auth",
				},
				{
					Name:        "token",
					Type:        tftypes.String,
					Required:    true,
					Sensitive:   true,
					Description: "The Artifactory API Key token or identity token. API keys are deprecated so it is best to use an identity token",
				},
				{
					Name:        "host",
					Type:        tftypes.String,
					Required:    true,
					Description: "The Artifactory API host. It should be the fully qualified base URL",
				},
				{
					Name:        "repo",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The Artifactory repository you want to search in",
				},
				{
					Name:        "path",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The sub-path inside the Artifactory repository to search in",
				},
				{
					Name:        "name",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of the artifact that you're looking for",
				},
				{
					Name:        "properties",
					Type:        tftypes.Map{ElementType: tftypes.String},
					Optional:    true,
					Description: "A map of properties to match on",
				},
				{
					Name:        "query_template",
					Type:        tftypes.String,
					Optional:    true,
					Description: "An AQL query to run. When a 'query' is provided all search properties are ignored so you must write the a complete and valid items.find() query",
				},
				{
					Name:            "results",
					Type:            s.Results.TFType(),
					Computed:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description: docCaretToBacktick(`
- ^results.name^ (String) The item name
- ^results.type^ (String) The item type
- ^results.url^ (String) The fully qualified URL to the item
- ^results.sha256^ (String) The SHA256 sum of the item
- ^results.size^ (String) The size of the item
`),
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

	if !s.QueryTemplate.Unknown {
		templ, ok := s.QueryTemplate.Get()
		if ok && templ != "" {
			_, err := template.New("query").Parse(templ)
			if err != nil {
				return ValidationError(
					fmt.Sprintf("invalid query template: %s", err.Error()),
					"query_template",
				)
			}
		}
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the		tftypes.Vault with As().
func (s *artifactoryItemStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":             s.ID,
		"username":       s.Username,
		"token":          s.Token,
		"host":           s.Host,
		"repo":           s.Repo,
		"path":           s.Path,
		"name":           s.Name,
		"properties":     s.Properties,
		"query_template": s.QueryTemplate,
		"results":        s.Results,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *artifactoryItemStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":             s.ID.TFType(),
		"username":       s.Username.TFType(),
		"token":          s.Token.TFType(),
		"host":           s.Host.TFType(),
		"repo":           s.Repo.TFType(),
		"path":           s.Path.TFType(),
		"name":           s.Name.TFType(),
		"properties":     s.Properties.TFType(),
		"query_template": s.QueryTemplate.TFType(),
		"results":        s.Results.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *artifactoryItemStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":             s.ID.TFValue(),
		"username":       s.Username.TFValue(),
		"token":          s.Token.TFValue(),
		"host":           s.Host.TFValue(),
		"repo":           s.Repo.TFValue(),
		"path":           s.Path.TFValue(),
		"name":           s.Name.TFValue(),
		"properties":     s.Properties.TFValue(),
		"query_template": s.QueryTemplate.TFValue(),
		"results":        s.Results.TFValue(),
	})
}

// Search queries the aritfactory API and parses the results.
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
	if query, ok := s.QueryTemplate.Get(); ok {
		if ok && query != "" {
			t, err := template.New("query").Parse(query)
			if err != nil {
				return fmt.Errorf("search failed: invalid query template: %w", err)
			}

			reqArgs = append(reqArgs, artifactory.WithLimit("1"))
			reqArgs = append(reqArgs, artifactory.WithQueryTemplate(t))
		}
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

		r.Set(map[string]any{
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
