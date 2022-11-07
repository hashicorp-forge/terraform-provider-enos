package plugin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/docker"
	"github.com/hashicorp/enos-provider/internal/kind"
	"github.com/hashicorp/enos-provider/internal/log"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type kindClientFactory func(logger log.Logger) kind.Client

var defaultClientFactory = func(logger log.Logger) kind.Client { return kind.NewLocalClient(logger) }

type localKindLoadImage struct {
	providerConfig *config
	mu             sync.Mutex
	clientFactory  kindClientFactory
}

func newLocalKindLoadImage() *localKindLoadImage {
	return &localKindLoadImage{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
		clientFactory:  defaultClientFactory,
	}
}

var _ resource.Resource = (*localKindLoadImage)(nil)

type localKindLoadImageStateV1 struct {
	ID           *tfString
	ClusterName  *tfString
	Image        *tfString
	Tag          *tfString
	Archive      *tfString
	LoadedImages *loadedImagesStateV1
}

var _ state.State = (*localKindLoadImageStateV1)(nil)

func newLocalKindLoadImageStateV1() *localKindLoadImageStateV1 {
	return &localKindLoadImageStateV1{
		ID:           newTfString(),
		ClusterName:  newTfString(),
		Image:        newTfString(),
		Tag:          newTfString(),
		Archive:      newTfString(),
		LoadedImages: newLoadedImagesStateV1(),
	}
}

type loadedImagesStateV1 struct {
	Repository *tfString
	Tag        *tfString
	Nodes      *tfStringSlice
}

var _ state.Serializable = (*loadedImagesStateV1)(nil)

func newLoadedImagesStateV1() *loadedImagesStateV1 {
	return &loadedImagesStateV1{
		Repository: newTfString(),
		Tag:        newTfString(),
		Nodes:      newTfStringSlice(),
	}
}

func (k *localKindLoadImage) Name() string {
	return "enos_local_kind_load_image"
}

func (k *localKindLoadImage) Schema() *tfprotov6.Schema {
	return newLocalKindLoadImageStateV1().Schema()
}

func (k *localKindLoadImage) SetProviderConfig(meta tftypes.Value) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.providerConfig.FromTerraform5Value(meta)
}

func (k *localKindLoadImage) GetProviderConfig() (*config, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.providerConfig.Copy()
}

func (k *localKindLoadImage) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	state := newLocalKindLoadImageStateV1()

	transportUtil.ValidateResourceConfig(ctx, state, req, res)
}

func (k *localKindLoadImage) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	transportUtil.UpgradeResourceState(ctx, newLocalKindLoadImageStateV1(), req, res)
}

func (k *localKindLoadImage) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	newState := newLocalKindLoadImageStateV1()

	err := unmarshal(newState, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	res.NewState, err = state.Marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

func (k *localKindLoadImage) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	transportUtil.ImportResourceState(ctx, newBoundaryInitStateV1(), req, res)
}

func (k *localKindLoadImage) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	priorState := newLocalKindLoadImageStateV1()
	proposedState := newLocalKindLoadImageStateV1()
	res.PlannedState = proposedState

	err := priorState.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	err = proposedState.FromTerraform5Value(req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.LoadedImages.Repository.Unknown = true
		proposedState.LoadedImages.Tag.Unknown = true
		proposedState.LoadedImages.Nodes.Unknown = true
	}

	res.RequiresReplace = []*tftypes.AttributePath{
		tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
			tftypes.AttributeName("cluster_name"),
		}),
		tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
			tftypes.AttributeName("image"),
		}),
		tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
			tftypes.AttributeName("tag"),
		}),
		tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
			tftypes.AttributeName("archive"),
		}),
	}
}

func (k *localKindLoadImage) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newLocalKindLoadImageStateV1()
	plannedState := newLocalKindLoadImageStateV1()
	res.NewState = plannedState

	err := plannedState.FromTerraform5Value(req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	// sets up a logger that will include the state in every log message
	logger := log.NewLogger(ctx).WithValues(map[string]interface{}{
		"id":      plannedState.ID.Value(),
		"cluster": plannedState.ClusterName.Value(),
		"image":   plannedState.Image.Value(),
		"tag":     plannedState.Tag.Value(),
		"archive": plannedState.Archive.Value(),
	})

	err = priorState.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	switch {
	case req.IsCreate():
		logger.Debug("Loading image into kind cluster")

		if err := plannedState.Validate(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Error", err))
			return
		}

		client := k.clientFactory(logger)

		var result kind.LoadedImageResult
		isLoadImageArchive := !plannedState.Archive.Null

		switch {
		case isLoadImageArchive:
			result, err = client.LoadImageArchive(kind.LoadImageArchiveRequest{
				ClusterName:  plannedState.ClusterName.Value(),
				ImageArchive: plannedState.Archive.Value(),
			})
			if err != nil {
				res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Load Archive Error", err))
				return
			}
		default:
			result, err = client.LoadImage(kind.LoadImageRequest{
				ClusterName: plannedState.ClusterName.Value(),
				ImageName:   plannedState.Image.Value(),
				Tag:         plannedState.Tag.Value(),
			})
			if err != nil {
				res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Load Image Error", err))
				return
			}
		}

		var loadedImage *docker.ImageRef
		for _, info := range result.Images {
			if info.Repository == plannedState.Image.Value() {
				for _, tagInfo := range info.Tags {
					if tagInfo.Tag == plannedState.Tag.Value() {
						loadedImage = &docker.ImageRef{
							Repository: info.Repository,
							Tag:        tagInfo.Tag,
						}
						break
					}
				}
			}
		}

		if loadedImage == nil {
			res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Image Load Failed",
				Detail:   "None of the loaded images match the configured image",
			})
			tflog.Error(ctx, "None of the loaded images match the configured image", map[string]interface{}{
				"image":         plannedState.Image.Value(),
				"tag":           plannedState.Tag.Value(),
				"loaded_images": fmt.Sprintf("%#v", result),
			})
			return
		}

		plannedState.ID.Set(plannedState.ClusterName.Value() + "-" + result.ID())
		plannedState.LoadedImages.Repository.Set(loadedImage.Repository)
		plannedState.LoadedImages.Tag.Set(loadedImage.Tag)
		plannedState.LoadedImages.Nodes.SetStrings(result.Nodes)

	case req.IsDelete():
		// we're not doing anything on delete, but maybe we should. The only time this would actually
		// matter would be for a long-lived kind cluster. If we ignore deleting images we could start
		// to use up too much disk space. As this is not the envisioned use case for a kind cluster,
		// this should not be an issue.
		logger.Debug("Deleting loaded image in kind clusters, not supported")

	case req.IsUpdate():
		logger.Debug("Updating loaded image in kind clusters, not supported")

		// an update should never happen since all the attributes of this resource if changed should
		// trigger a replace, rather than an update in place. See the PlanResourceChange implementation
		tflog.Warn(ctx, "Update not supported")
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Unexpected Resource Update",
			Detail:   "Image load resources cannot be updated in place.",
		})
	}
	// if you put anything here, it must be applicable for any of the create, update or delete cases
}

func (k localKindLoadImageStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Version: 1,
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:        "cluster_name",
					Type:        tftypes.String,
					Description: "The name of the cluster to load the image to",
					Required:    true,
				},
				{
					Name:        "image",
					Type:        tftypes.String,
					Description: "The name of the docker image to load without the tag, i.e. vault",
					Required:    true,
				},
				{
					Name:        "tag",
					Type:        tftypes.String,
					Description: "The tag of the docker image to load, i.e. 1.10.0",
					Required:    true,
				},
				{
					Name:        "archive",
					Type:        tftypes.String,
					Description: "An archive file to load, i.e. vault-1.10.0.tar",
					Optional:    true,
				},
				{
					Name:        "loaded_images",
					Type:        k.LoadedImages.Terraform5Type(),
					Description: "A list of node/image pairs for the images that where loaded",
					Computed:    true,
				},
			},
		},
	}
}

func (k localKindLoadImageStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	for name, attrib := range map[string]*tfString{
		"cluster_name": k.ClusterName,
		"image":        k.Image,
		"tag":          k.Tag,
		"archive":      k.Archive,
	} {
		if val, ok := attrib.Get(); ok && len(strings.TrimSpace(val)) == 0 {
			return ValidationError(fmt.Sprintf("'%s' attribute must contain a non-empty value", name), name)
		}
	}

	return nil
}

func (k localKindLoadImageStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":            k.ID,
		"cluster_name":  k.ClusterName,
		"image":         k.Image,
		"tag":           k.Tag,
		"archive":       k.Archive,
		"loaded_images": k.LoadedImages,
	})
	if err != nil {
		return fmt.Errorf("failed to convert Terraform value to kind image state, due to: %w", err)
	}
	return nil
}

func (k localKindLoadImageStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":            k.ID.TFType(),
		"cluster_name":  k.ClusterName.TFType(),
		"image":         k.Image.TFType(),
		"tag":           k.Tag.TFType(),
		"archive":       k.Archive.TFType(),
		"loaded_images": k.LoadedImages.Terraform5Type(),
	}}
}

func (k localKindLoadImageStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(k.Terraform5Type(), map[string]tftypes.Value{
		"id":            k.ID.TFValue(),
		"cluster_name":  k.ClusterName.TFValue(),
		"image":         k.Image.TFValue(),
		"tag":           k.Tag.TFValue(),
		"archive":       k.Archive.TFValue(),
		"loaded_images": k.LoadedImages.Terraform5Value(),
	})
}

func (l *loadedImagesStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"repository": l.Repository.TFType(),
		"tag":        l.Tag.TFType(),
		"nodes":      l.Nodes.TFType(),
	}}
}

func (l *loadedImagesStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(l.Terraform5Type(), map[string]tftypes.Value{
		"repository": l.Repository.TFValue(),
		"tag":        l.Tag.TFValue(),
		"nodes":      l.Nodes.TFValue(),
	})
}

func (l *loadedImagesStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"repository": l.Repository,
		"tag":        l.Tag,
		"nodes":      l.Nodes,
	})
	if err != nil {
		return AttributePathError(
			fmt.Errorf("failed to convert Terraform Value to loaded images state, due to: %w", err),
			"loaded_images",
		)
	}
	return nil
}

func (k localKindLoadImageStateV1) Debug() string {
	return ""
}
