package artifactory

// supportedVaultTargets is our currently supported Vault build targets.
var supportedVaultTargets = map[string][]string{
	"darwin":  []string{"amd64"},
	"freebsd": []string{"386", "amd64", "arm"},
	"linux":   []string{"386", "amd64", "arm", "arm64"},
	"netbsd":  []string{"386", "amd64"},
	"openbsd": []string{"386", "amd64", "arm"},
	"solaris": []string{"amd64"},
	"windows": []string{"386", "amd64"},
}

// supportedEditions is our currently supported Vault editions
var supportedVaultEditions = []string{"oss", "ent", "ent.hsm", "prem", "prem.hsm", "pro"}

// SupportedVaultArch validates that the given platform and arch are supported
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

// SupportedVaultPlatform validates that the given platform is supported
func SupportedVaultPlatform(platform string) bool {
	_, ok := supportedVaultTargets[platform]
	return ok
}

// SupportedVaultEdition validates that the given edition is a valid for Vault
func SupportedVaultEdition(ed string) bool {
	for _, edition := range supportedVaultEditions {
		if ed == edition {
			return true
		}
	}

	return false
}
