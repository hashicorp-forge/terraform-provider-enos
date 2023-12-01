package plugin

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/enos-provider/internal/docker"
	"github.com/hashicorp/enos-provider/internal/kind"
	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var (
	clusterName = "funky-chicken"
	image       = "bananas"
	tag         = "0.1.0"
	archive     = "docker_image.tar"
	cfg         = template.Must(template.New("kind_cluster").Parse(`resource "enos_local_kind_load_image" "bananas" {
		{{if .ClusterName.Value}}
		cluster_name = "{{.ClusterName.Value}}"
		{{end}}
		{{if .Image.Value}}
		image = "{{.Image.Value}}"
		{{end}}
		{{if .Tag.Value}}
		tag = "{{.Tag.Value}}"
		{{end}}
		{{if .Archive.Value}}
		archive = "{{.Archive.Value}}"
		{{end}}
	}`))
)

func TestAccResourceKindLoadImage(t *testing.T) {
	t.Parallel()

	loadImageState := newLocalKindLoadImageStateV1()
	loadImageState.Image.Set(image)
	loadImageState.Tag.Set(tag)
	loadImageState.ClusterName.Set(clusterName)
	testLoadImage := testAccResourceTemplate{
		name:  "valid_attributes_load_image",
		state: loadImageState,
		check: resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "id", regexp.MustCompile(fmt.Sprintf(`^%s-\d+$`, clusterName))),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "image", regexp.MustCompile(image)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "tag", regexp.MustCompile(tag)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "cluster_name", regexp.MustCompile(clusterName)),
		),
		apply: true,
	}

	loadImageArchiveState := newLocalKindLoadImageStateV1()
	loadImageArchiveState.Image.Set(image)
	loadImageArchiveState.Tag.Set(tag)
	loadImageArchiveState.ClusterName.Set(clusterName)
	loadImageArchiveState.Archive.Set(archive)
	testLoadImageArchive := testAccResourceTemplate{
		name:  "valid_attributes_load_image_archive",
		state: loadImageState,
		check: resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "id", regexp.MustCompile(fmt.Sprintf(`^%s-\d+$`, clusterName))),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "image", regexp.MustCompile(image)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "tag", regexp.MustCompile(tag)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "cluster_name", regexp.MustCompile(clusterName)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "archive", regexp.MustCompile(archive)),
		),
		apply: true,
	}

	loadImageStateInvalid := newLocalKindLoadImageStateV1()
	loadImageStateInvalid.Image.Set(image)
	loadImageStateInvalid.Tag.Set(tag)
	loadImageStateInvalid.ClusterName.Set(clusterName)
	loadImageStateInvalid.Archive.Set("  ")
	testLoadImageInvalid := testAccResourceTemplate{
		name:  "invalid_load_archive",
		state: loadImageStateInvalid,
		check: resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "id", regexp.MustCompile(fmt.Sprintf(`^%s-\d+$`, clusterName))),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "image", regexp.MustCompile(image)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "tag", regexp.MustCompile(tag)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "cluster_name", regexp.MustCompile(clusterName)),
			resource.TestMatchResourceAttr("enos_local_kind_load_image.bananas", "  ", regexp.MustCompile(archive)),
		),
		apply: true,
	}

	//nolint:paralleltest// because our resource handles it
	for _, test := range []struct {
		tmpl                            testAccResourceTemplate
		state                           localKindLoadImageStateV1
		expectedLoadImageRequest        *kind.LoadImageRequest
		expectedLoadImageArchiveRequest *kind.LoadImageArchiveRequest
		expectedErr                     *regexp.Regexp
	}{
		{
			testLoadImage,
			*loadImageState,
			&kind.LoadImageRequest{
				ClusterName: clusterName,
				ImageName:   image,
				Tag:         tag,
			},
			nil,
			nil,
		},
		{
			testLoadImageArchive,
			*loadImageArchiveState,
			nil,
			&kind.LoadImageArchiveRequest{
				ClusterName:  clusterName,
				ImageArchive: archive,
			},
			nil,
		},
		{
			testLoadImageInvalid,
			*loadImageStateInvalid,
			nil,
			nil,
			regexp.MustCompile(`Validation Error`),
		},
	} {
		test := test
		t.Run(test.tmpl.name, func(tt *testing.T) {
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			tmpl := test.tmpl

			step := resource.TestStep{
				Config:             buf.String(),
				Check:              tmpl.check,
				ExpectNonEmptyPlan: !tmpl.apply,
				PlanOnly:           !tmpl.apply,
				ExpectError:        test.expectedErr,
			}

			kindLoadImage := newLocalKindLoadImage()
			mockKindClient := NewMockKindClient()
			kindLoadImage.clientFactory = func(logger log.Logger) kind.Client { return mockKindClient }

			providers := testProviders(t, providerOverrides{resources: []resourcerouter.Resource{kindLoadImage}})

			resource.ParallelTest(tt, resource.TestCase{
				ProtoV6ProviderFactories: providers,
				Steps:                    []resource.TestStep{step},
			})

			if test.expectedErr != nil {
				// check that there was one call to load an image
				if test.expectedLoadImageRequest != nil {
					assert.Len(tt, mockKindClient.loadImageRequests, 1)
					assert.Equal(tt, mockKindClient.loadImageRequests[0], *test.expectedLoadImageRequest)
					assert.Empty(tt, mockKindClient.loadImageArchiveRequests)
				} else if test.expectedLoadImageArchiveRequest != nil {
					assert.Len(tt, mockKindClient.loadImageArchiveRequests, 1)
					assert.Equal(tt, mockKindClient.loadImageArchiveRequests[0], *test.expectedLoadImageArchiveRequest)
					assert.Empty(tt, mockKindClient.loadImageRequests)
				}

				// sanity check
				assert.Empty(tt, mockKindClient.createRequests)
				assert.Empty(tt, mockKindClient.deleteRequests)
			}
		})
	}
}

type MockKindClient struct {
	createRequests           []kind.CreateKindClusterRequest
	deleteRequests           []kind.DeleteKindClusterRequest
	loadImageArchiveRequests []kind.LoadImageArchiveRequest
	loadImageRequests        []kind.LoadImageRequest
}

func NewMockKindClient() *MockKindClient {
	return &MockKindClient{
		createRequests:           []kind.CreateKindClusterRequest{},
		deleteRequests:           []kind.DeleteKindClusterRequest{},
		loadImageArchiveRequests: []kind.LoadImageArchiveRequest{},
		loadImageRequests:        []kind.LoadImageRequest{},
	}
}

func (m *MockKindClient) CreateCluster(request kind.CreateKindClusterRequest) (kind.ClusterInfo, error) {
	m.createRequests = append(m.createRequests, request)
	return kind.ClusterInfo{
		KubeConfigBase64:     "fee",
		ContextName:          "kind" + request.Name,
		ClientCertificate:    "fi",
		ClientKey:            "fo",
		ClusterCACertificate: "fum",
		Endpoint:             "foo",
	}, nil
}

func (m *MockKindClient) DeleteCluster(request kind.DeleteKindClusterRequest) error {
	m.deleteRequests = append(m.deleteRequests, request)
	return nil
}

func (m *MockKindClient) LoadImageArchive(req kind.LoadImageArchiveRequest) (kind.LoadedImageResult, error) {
	m.loadImageArchiveRequests = append(m.loadImageArchiveRequests, req)

	// this is a bogus return value, don't use it to do any validation
	loadImageArchiveResponse := kind.LoadedImageResult{
		Images: []docker.ImageInfo{{
			Repository: image,
			Tags: []docker.TagInfo{
				{
					Tag: tag,
					ID:  "123456",
				},
			},
		}},
	}

	return loadImageArchiveResponse, nil
}

func (m *MockKindClient) LoadImage(req kind.LoadImageRequest) (kind.LoadedImageResult, error) {
	m.loadImageRequests = append(m.loadImageRequests, req)
	return kind.LoadedImageResult{
		Images: []docker.ImageInfo{{
			Repository: req.ImageName,
			Tags: []docker.TagInfo{
				{
					Tag: req.Tag,
					ID:  "123456", // not a valid id
				},
			},
		}},
	}, nil
}
