# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "host_public_ip" {
  description = "The public IP address of the remote host where Vault and Consul are already running"
  type        = string
}

variable "host_private_ip" {
  description = "The private IP address of the remote host (used for Vault cluster_addr / api_addr)"
  type        = string
}

variable "debug_data_root_dir" {
  description = "Local directory where the enos provider writes failure-handler log files. Must match the debug_data_root_dir configured on the provider."
  type        = string
}
