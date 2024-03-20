# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "results" {
  value = data.enos_artifactory_item.vault.results
}
