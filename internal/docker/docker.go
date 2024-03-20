// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package docker

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// ImageInfo information about a docker image.
type ImageInfo struct {
	Repository string
	Tags       []TagInfo
}

// TagInfo information about an image tag.
type TagInfo struct {
	Tag string
	// ID docker image ID
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

var repositoriesFile = "repositories"

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
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image archive: %s", archivePath)
	}
	defer archiveFile.Close()

	archiveReader := tar.NewReader(archiveFile)

	for {
		header, err := archiveReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("failed to find docker file manifest")
			}

			return nil, fmt.Errorf("failed to read archive contents due to: %w", err)
		}
		fileInfo := header.FileInfo()
		if fileInfo.Name() == repositoriesFile {
			repositories, err := io.ReadAll(archiveReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s file due to: %w", repositoriesFile, err)
			}

			// repository --> tag --> id
			var imageInfo map[string]map[string]string
			err = json.Unmarshal(repositories, &imageInfo)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal %s file, due to: %w", repositoriesFile, err)
			}

			var infos []ImageInfo
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

			return infos, nil
		}
	}
}
