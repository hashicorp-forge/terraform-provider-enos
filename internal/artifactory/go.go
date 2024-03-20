// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package artifactory

// supportedVaultTargets is our currently supported Vault build targets.
var supportedVaultTargets = map[string][]string{
	"darwin":  {"amd64"},
	"freebsd": {"386", "amd64", "arm"},
	"linux":   {"386", "amd64", "arm", "arm64"},
	"netbsd":  {"386", "amd64"},
	"openbsd": {"386", "amd64", "arm"},
	"solaris": {"amd64"},
	"windows": {"386", "amd64"},
}

// supportedEditions is our currently supported Vault editions.
var supportedVaultEditions = []string{"ce", "oss", "ent", "ent.hsm", "ent.fips1402", "ent.hsm.fips1402"}

// SupportedVaultArch validates that the given platform and arch are supported.
func SupportedVaultArch(platform, arch string) bool {
	archs, ok := supportedVaultTargets[platform]
	if !ok {
		return false
	}

	for _, a := range archs {
		if arch == a {
			return true
		}
	}

	return false
}

// SupportedVaultPlatform validates that the given platform is supported.
func SupportedVaultPlatform(platform string) bool {
	_, ok := supportedVaultTargets[platform]
	return ok
}

// SupportedVaultEdition validates that the given edition is a valid for Vault.
func SupportedVaultEdition(ed string) bool {
	for _, edition := range supportedVaultEditions {
		if ed == edition {
			return true
		}
	}

	return false
}
