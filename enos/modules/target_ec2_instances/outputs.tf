# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "cluster_name" {
  value = local.cluster_name
}

output "hosts" {
  description = "The ec2 instance target hosts"
  value       = local.hosts
}
