package docker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImageInfos(t *testing.T) {
	t.Parallel()

	for name, expected := range map[string][]ImageInfo{
		"docker_repositories_file": {
			{
				Repository: "hashicorp/vault-enterprise",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
			{
				Repository: "public.ecr.aws/hashicorp/vault-enterprise",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
			{
				Repository: "vault-enterprise/default/linux/arm64",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent_e13abd86cad97d6bc6490507d9e0960ab324e6d3",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
		},
		"oci_spec": {
			{
				Repository: "docker.io/hashicorp/vault-enterprise",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
			{
				Repository: "public.ecr.aws/hashicorp/vault-enterprise",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
			{
				Repository: "docker.io/vault-enterprise/default/linux/arm64",
				Tags: []TagInfo{
					{
						Tag: "1.22.0-beta1-ent_e13abd86cad97d6bc6490507d9e0960ab324e6d3",
						ID:  "9eefd28c43dc5a6be3f9c36642b5497954f1e7afa35b11410c25270f01218ac0",
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path, err := filepath.Abs(filepath.Join("..", "fixtures", name+".tar"))
			require.NoError(t, err)
			infos, err := GetImageInfos(path)
			require.NoError(t, err)
			require.Equal(t, sortImageInfos(expected), infos)
		})
	}
}
