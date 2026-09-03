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

