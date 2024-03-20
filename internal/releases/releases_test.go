// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package releases

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBundleURL(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Rel      *Release
		Expected string
	}{
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "oss",
				Platform: "linux",
				Arch:     "arm64",
			},
			"https://releases.hashicorp.com/vault/1.7.0/vault_1.7.0_linux_arm64.zip",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.15.0",
				Edition:  "ce",
				Platform: "linux",
				Arch:     "arm64",
			},
			"https://releases.hashicorp.com/vault/1.15.0/vault_1.15.0_linux_arm64.zip",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "ent.hsm",
				Platform: "linux",
				Arch:     "amd64",
			},
			"https://releases.hashicorp.com/vault/1.7.0+ent.hsm/vault_1.7.0+ent.hsm_linux_amd64.zip",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "ent",
				Platform: "freebsd",
				Arch:     "386",
			},
			"https://releases.hashicorp.com/vault/1.7.0+ent/vault_1.7.0+ent_freebsd_386.zip",
		},
	} {
		test := test
		rel := test.Rel
		t.Run(fmt.Sprintf("%s_%s_%s_%s_%s", rel.Product, rel.Version, rel.Edition, rel.Platform, rel.Arch), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.Expected, rel.BundleURL())
		})
	}
}

var testEntSHASums = `0c7e49ecc0b00202a515f2e819664850aad0ff617991aec03589b725a0540880  vault_1.7.0+ent_darwin_amd64.zip
e88e8cbaa8e116a5bc2d4fb541b63de212d2c3414772af1d39f258ed5f982d71  vault_1.7.0+ent_freebsd_386.zip
08b36b9c0c09499c946676bae13c54cadb04493971943959c36406308a238c5f  vault_1.7.0+ent_freebsd_amd64.zip
4a58ea1012b9ca0454acf411aa2db3a1f3be003a1a7a76e93137b9cfc1381b99  vault_1.7.0+ent_freebsd_arm.zip
550e5c5ebdb55a27738932bfd8b1efc3d22b73a62c3921d4a5e5c48947867f5d  vault_1.7.0+ent_linux_386.zip
d01a82111133908167a5a140604ab3ec8fd18601758376a5f8e9dd54c7703373  vault_1.7.0+ent_linux_amd64.zip
23ce4d2bce8088c7a6e2970e113fe6d5bff6092da1de1c1c967bca3d55d6bdcb  vault_1.7.0+ent_linux_arm.zip
96a398c8ceb0830a875070adaf0dafe64f4e20c458b3876102ee8100ee0fb80f  vault_1.7.0+ent_linux_arm64.zip
a1eb91e833a9f3bb7e6def38c74e5a7ef49e49d6bdad955af47c4b3414aadc65  vault_1.7.0+ent_netbsd_386.zip
2a97c8a73582fd558bc253733cc30c6174f407f79e3ab91a0414b1654b3e9a1e  vault_1.7.0+ent_netbsd_amd64.zip
c4af21b25976a13a3069df62d232f2503c33a3d2ab6d8f25e45668dd396a5ef9  vault_1.7.0+ent_openbsd_386.zip
c08b012069637a2db7e457ec3db703d465747194c6be4328f095f526accf095a  vault_1.7.0+ent_openbsd_amd64.zip
af1f53daacc5b82ee353de5ca0779c341cdcaece21837ba0c9997f48ae1b44a6  vault_1.7.0+ent_openbsd_arm.zip
f46ea11dbd06426e4bdaaed41a2de08270b33371076390a54225f222cf0b5117  vault_1.7.0+ent_solaris_amd64.zip
6b153860be3a1f82d16e6a7dad58b894082c4c083b24a87dccb8e989baef1d3c  vault_1.7.0+ent_windows_386.zip
355416ff4e191935c87a166b1a754b9e7fee5073c8482e9f6537a74d188a5bc0  vault_1.7.0+ent_windows_amd64.zip
`

var testOSSSHASums = `1d59ff9054496ccd2c4f803ce599a163794f7455764b186453fe7e975606b2a0  vault_1.7.0_darwin_amd64.zip
d87871390363add49c4c432aed43c8b53fbe74e83a7101920ef0c83b49f18d55  vault_1.7.0_freebsd_386.zip
04e45b4f475c3b67cfa6c33327d37dd00384129592d1b7bd62d8790b804c6ead  vault_1.7.0_freebsd_amd64.zip
65183e85fce98b5fe0eed08af95e656198a9c61272a1afa780ee2d922764186f  vault_1.7.0_freebsd_arm.zip
cd134d06ca888ec92372c9b7483d1e2a8669abc64845f0bdba2cc452e1051989  vault_1.7.0_linux_386.zip
aad2f50635ef4e3f2495b0b6c855061c4047c795821fda886b326c1a71c71c35  vault_1.7.0_linux_amd64.zip
b92f2fe00bdcc7b0046075d6132eca83123c11b286a8ca4897e4fb6008bf063c  vault_1.7.0_linux_arm.zip
ef5fd091c40452e4cd5c855de6cc85a6c9654790e707b342a0cb9fef48d80d2a  vault_1.7.0_linux_arm64.zip
41c40bab02313ee42650ae37b8ca3df991836d46b758dc33f66105b5a4b6a5f4  vault_1.7.0_netbsd_386.zip
962f189918cff6e6252f829304647fcf896c9d102a790999871a8c5e67379641  vault_1.7.0_netbsd_amd64.zip
bb6a06f83b5d0396f14f1ced437a3220c238aacfa403ce91dd9071bcd6466c9b  vault_1.7.0_openbsd_386.zip
33caa17a5654c7be2395389706c466b97bd03c54c0e261726d88920a5abe75a2  vault_1.7.0_openbsd_amd64.zip
5db1b07786963593e808d116cd26f7a5ede5a4f32554c6d1daff64ddb7a3050f  vault_1.7.0_solaris_amd64.zip
00f5ebf7660ff93013cfb7c1d904e0fb7feb909cdf22b9fafd78d058b6898c52  vault_1.7.0_windows_386.zip
b192e31e2c0ddc001ca39b25fb4b3c99e916f41e7dc2713ab1b542ce7304bf37  vault_1.7.0_windows_amd64.zip
`

func TestSHA256(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Rel      *Release
		Expected string
	}{
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "oss",
				Platform: "linux",
				Arch:     "arm64",
			},
			"ef5fd091c40452e4cd5c855de6cc85a6c9654790e707b342a0cb9fef48d80d2a",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "oss",
				Platform: "linux",
				Arch:     "amd64",
			},
			"aad2f50635ef4e3f2495b0b6c855061c4047c795821fda886b326c1a71c71c35",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "oss",
				Platform: "darwin",
				Arch:     "amd64",
			},
			"1d59ff9054496ccd2c4f803ce599a163794f7455764b186453fe7e975606b2a0",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "ent",
				Platform: "linux",
				Arch:     "arm64",
			},
			"96a398c8ceb0830a875070adaf0dafe64f4e20c458b3876102ee8100ee0fb80f",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "ent",
				Platform: "linux",
				Arch:     "amd64",
			},
			"d01a82111133908167a5a140604ab3ec8fd18601758376a5f8e9dd54c7703373",
		},
		{
			&Release{
				Product:  "vault",
				Version:  "1.7.0",
				Edition:  "ent",
				Platform: "darwin",
				Arch:     "amd64",
			},
			"0c7e49ecc0b00202a515f2e819664850aad0ff617991aec03589b725a0540880",
		},
	} {
		test := test
		rel := test.Rel
		t.Run(fmt.Sprintf("%s_%s_%s", rel.Edition, rel.Platform, rel.Arch), func(t *testing.T) {
			t.Parallel()
			if rel.Edition == "ent" {
				rel.GetSHA256Sums = func(*Release) (string, error) {
					return testEntSHASums, nil
				}
			} else {
				rel.GetSHA256Sums = func(*Release) (string, error) {
					return testOSSSHASums, nil
				}
			}
			sha, err := rel.SHA256()
			require.NoError(t, err)
			require.Equal(t, test.Expected, sha)
		})
	}
}

func TestAccDefaultGetSHA256Sums(t *testing.T) {
	t.Parallel()

	_, okacc := os.LookupEnv("TF_ACC")
	if !okacc {
		t.Log("skipping because TF_ACC is required to run acceptance tests")
		t.Skip()
	}

	rel, err := NewRelease(
		WithReleaseProduct("vault"),
		WithReleaseVersion("1.7.0"),
		WithReleaseEdition("ent"),
		WithReleasePlatform("linux"),
		WithReleaseArch("arm64"),
	)
	require.NoError(t, err)

	sums, err := rel.GetSHA256Sums(rel)
	require.NoError(t, err)
	require.Equal(t, testEntSHASums, sums)
}
