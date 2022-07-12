package plugin

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/hashicorp/enos-provider/internal/kind"
	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type localKindLoadImage struct {
	providerConfig *config
	mu             sync.Mutex
}

func newLocalKindLoadImage() *localKindLoadImage {
	return &localKindLoadImage{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

var _ resourcerouter.Resource = (*localKindLoadImage)(nil)

type localKindLoadImageStateV1 struct {
	ID           *tfString
	ClusterName  *tfString
	Image        *tfString
	Tag          *tfString
	LoadedImages *loadedImagesStateV1
}

var _ State = (*localKindLoadImageStateV1)(nil)

func newLocalKindLoadImageStateV1() *localKindLoadImageStateV1 {
	return &localKindLoadImageStateV1{
		ID:           newTfString(),
		ClusterName:  newTfString(),
		Image:        newTfString(),
		Tag:          newTfString(),
		LoadedImages: newLoadedImagesStateV1(),
	}
}

type loadedImagesStateV1 struct {
	Image *tfString
	Nodes *tfStringSlice
}

var _ Serializable = (*loadedImagesStateV1)(nil)

func newLoadedImagesStateV1() *loadedImagesStateV1 {
	return &loadedImagesStateV1{
		Image: newTfString(),
		Nodes: newTfStringSlice(),
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
	transportUtil.ValidateResourceConfig(ctx, newLocalKindLoadImageStateV1(), req, res)
}

func (k *localKindLoadImage) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	transportUtil.UpgradeResourceState(ctx, newLocalKindLoadImageStateV1(), req, res)
}

func (k *localKindLoadImage) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newLocalKindLoadImageStateV1()

	err := unmarshal(newState, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	res.NewState, err = marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}
}

func (k *localKindLoadImage) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	transportUtil.ImportResourceState(ctx, newBoundaryInitStateV1(), req, res)
}

func (k *localKindLoadImage) PlanResourceChange(ctx context.Context, req tfprotov6.PlanResourceChangeRequest, res *tfprotov6.PlanResourceChangeResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	priorState := newLocalKindLoadImageStateV1()
	proposedState := newLocalKindLoadImageStateV1()

	err := unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	err = unmarshal(proposedState, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.LoadedImages.Image.Unknown = true
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
	}

	res.PlannedState, err = marshal(proposedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
}

func (k *localKindLoadImage) ApplyResourceChange(ctx context.Context, req tfprotov6.ApplyResourceChangeRequest, res *tfprotov6.ApplyResourceChangeResponse) {
	priorState := newLocalKindLoadImageStateV1()
	plannedState := newLocalKindLoadImageStateV1()

	err := unmarshal(plannedState, req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	// sets up a logger that will include the state in every log message
	logger := log.NewLogger(ctx).WithValues(map[string]interface{}{
		"id":      plannedState.ID.Value(),
		"cluster": plannedState.ClusterName.Value(),
		"infos":   plannedState.Image.Value(),
		"tag":     plannedState.Tag.Value(),
	})

	err = unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	isCreate := priorState.ID.Val == ""
	isDelete := plannedState.Image.Val == ""
	isUpdate := !isCreate && !isDelete && reflect.DeepEqual(plannedState, priorState)

	switch {
	case isCreate:
		logger.Debug("Loading image into kind cluster")

		client := kind.NewLocalClient(logger)
		loadImageRequest := kind.LoadImageRequest{
			ClusterName: plannedState.ClusterName.Value(),
			ImageName:   plannedState.Image.Value(),
			Tag:         plannedState.Tag.Value(),
		}
		infos, err := client.LoadImage(loadImageRequest)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return
		}

		plannedState.ID.Set(plannedState.ClusterName.Value() + "-" + loadImageRequest.GetImageRef())
		plannedState.LoadedImages.Image.Set(infos.Image)
		plannedState.LoadedImages.Nodes.SetStrings(infos.Nodes)

		if res.NewState, err = marshal(plannedState); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}

	case isDelete:
		// we're not doing anything on delete, but maybe we should. The only time this would actually
		// matter would be for a long-lived kind cluster. If we ignore deleting images we could start
		// to use up too much disk space. As this is not the envisioned use case for a kind cluster,
		// this should not be an issue.
		logger.Debug("Deleting loaded image in kind clusters, not supported")

		plannedState.ID.Set("")
		if res.NewState, err = marshalDelete(plannedState); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}

	case isUpdate:
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
					Description: "The tag of the docker image to load without the tag, i.e. vault",
					Required:    true,
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
	} {
		if val, ok := attrib.Get(); ok && len(val) == 0 {
			return newErrWithDiagnostics("Invalid Configuration", fmt.Sprintf("'%s' attribute must contain a non-empty value", name), name)
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
		"loaded_images": k.LoadedImages,
	})
	if err != nil {
		return wrapErrWithDiagnostics(err, "Error", "Failed to convert Terraform Value to kind load image state.")
	}
	return nil
}

func (k localKindLoadImageStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":            k.ID.TFType(),
		"cluster_name":  k.ClusterName.TFType(),
		"image":         k.Image.TFType(),
		"tag":           k.Tag.TFType(),
		"loaded_images": k.LoadedImages.Terraform5Type(),
	}}
}

func (k localKindLoadImageStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(k.Terraform5Type(), map[string]tftypes.Value{
		"id":            k.ID.TFValue(),
		"cluster_name":  k.ClusterName.TFValue(),
		"image":         k.Image.TFValue(),
		"tag":           k.Tag.TFValue(),
		"loaded_images": k.LoadedImages.Terraform5Value(),
	})
}

func (l *loadedImagesStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"image": l.Image.TFType(),
		"nodes": l.Nodes.TFType(),
	}}
}

func (l *loadedImagesStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(l.Terraform5Type(), map[string]tftypes.Value{
		"image": l.Image.TFValue(),
		"nodes": l.Nodes.TFValue(),
	})
}

func (l *loadedImagesStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"image": l.Image,
		"nodes": l.Nodes,
	})
	if err != nil {
		return wrapErrWithDiagnostics(err, "Error", "Failed to convert Terraform Value to loaded images state.", "loaded_images")
	}
	return nil
}
