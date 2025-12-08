// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"regexp"
	"strings"
)

// ServiceInfo is a list units of type service from systemctl command reference https://man7.org/linux/man-pages/man1/systemctl.1.html#COMMANDS
type ServiceInfo struct {
	Unit        string
	Load        string
	Active      string
	Sub         string
	Description string
}

// serviceInfoRegex is the regex used to parse the systemctl services output into a slice of ServiceInfo.
var serviceInfoRegex = regexp.MustCompile(`^(?P<unit>\S+)\.service\s+(?P<load>\S+)\s+(?P<active>\S+)\s+(?P<sub>\S+)\s+(?P<description>\S.*)$`)

// parseServiceInfos parses the systemctl services output into a slice of ServiceInfos.
func parseServiceInfos(services string) []ServiceInfo {
	serviceInfos := []ServiceInfo{}
	for line := range strings.SplitSeq(services, "\n") {
		if line == "" {
			continue
		}
		match := serviceInfoRegex.FindStringSubmatch(line)
		result := make(map[string]string)
		for i, name := range serviceInfoRegex.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}
		serviceInfos = append(serviceInfos, ServiceInfo{
			Unit:        result["unit"],
			Load:        result["load"],
			Active:      result["active"],
			Sub:         result["sub"],
			Description: result["description"],
		})
	}

	return serviceInfos
}
