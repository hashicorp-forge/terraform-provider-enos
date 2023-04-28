package flightcontrol

import (
	"embed"
	"fmt"
	"regexp"
)

// Binaries are the embedded enos-flight-control binaries
//
//go:embed binaries/*
var Binaries embed.FS

var binaryRegex = regexp.MustCompile(`enos-flight-control_(?P<platform>\w*)_(?P<arch>\w*)$`)

// SupportedTargets are the supported platform and architecture combinations that
// have been embedded.
func SupportedTargets() (map[string][]string, error) {
	targets := map[string][]string{}

	entries, err := Binaries.ReadDir("binaries")
	if err != nil {
		return targets, err
	}

	for _, entry := range entries {
		// this should never happen
		if entry.IsDir() {
			continue
		}

		matches := binaryRegex.FindStringSubmatch(entry.Name())
		if len(matches) != 3 {
			continue
		}

		platform := matches[1]
		arch := matches[2]

		_, ok := targets[platform]
		if !ok {
			targets[platform] = []string{}
		}

		targets[platform] = append(targets[platform], arch)
	}

	return targets, nil
}

// SupportedTarget checks to see is a target is supported.
func SupportedTarget(platform, arch string) (bool, error) {
	supportedTargets, err := SupportedTargets()
	if err != nil {
		return false, err
	}

	archs, ok := supportedTargets[platform]
	if !ok {
		return false, nil
	}

	for _, a := range archs {
		if arch == a {
			return true, nil
		}
	}

	return false, nil
}

// ReadTargetFile takes a platform and arch and attempts to read the file.
func ReadTargetFile(platform, arch string) ([]byte, error) {
	return Binaries.ReadFile(fmt.Sprintf("binaries/enos-flight-control_%s_%s", platform, arch))
}
