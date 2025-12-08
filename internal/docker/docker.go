// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package docker

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cpuguy83/tar2go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImageInfo information about a docker image.
type ImageInfo struct {
	Repository string
	Tags       []TagInfo
}

// TagInfo information about an image tag.
type TagInfo struct {
	Tag string
	// ID is the docker image ID (final layer hash of the image).
	ID string
}

// ImageRef the repo and tag for one image.
type ImageRef struct {
	Repository string
	Tag        string
}

func (i *ImageRef) String() string {
	return fmt.Sprintf("%s:%s", i.Repository, i.Tag)
}

func (i *ImageInfo) String() string {
	return fmt.Sprintf("%#v", i)
}

// TODO: do we need this function
// GetImageRefs creates a slice of all the image refs, where an image ref is Repository:Tag.
func (i *ImageInfo) GetImageRefs() []string {
	refs := make([]string, len(i.Tags))
	for idx, tag := range i.Tags {
		refs[idx] = fmt.Sprintf("%s:%s", i.Repository, tag)
	}

	return refs
}

func GetImageInfos(archivePath string) ([]ImageInfo, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}

	tfs := tar2go.NewIndex(f).FS()

	// Try Docker v1.1 as it has historically been what we've done for determining
	// our image information.
	rf, err := tfs.Open("repositories")
	if err == nil {
		return getImageInfoDockerMetadata(rf)
	}

	// Docker v1.1 didn't work. The container was probably built with Docker
	// >=29.0.0 so it's no longer there. Try OCI v1.
	return getImageInfoOCIMetadata(tfs)
}

func getImageInfoDockerMetadata(repoFile fs.File) ([]ImageInfo, error) {
	repositories, err := io.ReadAll(repoFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read container image 'repositories' file: %w", err)
	}

	// repository --> tag --> id
	var imageInfo map[string]map[string]string
	err = json.Unmarshal(repositories, &imageInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal container image 'repositories' file: %w", err)
	}

	infos := []ImageInfo{}
	for repo, info := range imageInfo {
		var tags []TagInfo
		for tag, id := range info {
			tags = append(tags, TagInfo{
				Tag: tag,
				ID:  id,
			})
		}
		infos = append(infos, ImageInfo{
			Repository: repo,
			Tags:       tags,
		})
	}

	return sortImageInfos(infos), nil
}

func getImageInfoOCIMetadata(tfs fs.FS) ([]ImageInfo, error) {
	idxb, err := fs.ReadFile(tfs, "index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read container image OCI 'index.json' file: %w", err)
	}

	var idx ocispec.Index
	err = json.Unmarshal(idxb, &idx)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal container image OCI 'index.json' file: %w", err)
	}

	// Iterate through the manifests and create our information.
	if len(idx.Manifests) < 1 {
		return nil, errors.New("container 'index.json' file does not contain any manifests")
	}
	var infos []ImageInfo
	resolvedIDS := map[string]string{}

	for _, manifest := range idx.Manifests {
		manifestDigest := manifest.Digest.Encoded()

		// Get the container ID. First, find the manifest blob and then get the config
		// layer SHA from it.
		id, ok := resolvedIDS[manifestDigest]
		if !ok {
			manb, err := fs.ReadFile(tfs, filepath.Join("blobs", "sha256", manifestDigest))
			if err != nil {
				return nil, fmt.Errorf("failed to read container manifest blob '%s': %w", manifestDigest, err)
			}

			var man ocispec.Manifest
			err = json.Unmarshal(manb, &man)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal container manifest blob '%s': %w", manifestDigest, err)
			}

			// The identifier of the image is the SHA256 of the "config" layer.
			id = man.Config.Digest.Encoded()
			resolvedIDS[manifestDigest] = id
		}

		for name, value := range manifest.Annotations {
			if name != "io.containerd.image.name" {
				continue
			}

			// Split the image name into the repository and the tag
			// e.g: docker.io/hashicorp/vault-enterprise:1.22.0-beta1-ent
			parts := strings.SplitN(value, ":", 2)

			// Add our ImageInfo to infos
			infos = append(infos, ImageInfo{
				Repository: parts[0],
				Tags: []TagInfo{
					{
						ID:  id,
						Tag: parts[1],
					},
				},
			})
		}
	}

	return sortImageInfos(infos), nil
}

// sortImageInfos sorts ImageInfo by repository name.
func sortImageInfos(in []ImageInfo) []ImageInfo {
	if len(in) < 2 {
		return in
	}

	slices.SortStableFunc(in, func(a ImageInfo, b ImageInfo) int {
		return cmp.Or(
			strings.Compare(a.Repository, b.Repository),
			cmp.Compare(len(a.Tags), len(b.Tags)),
		)
	})

	return in
}
