# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

variable "hosts" {
  type = map(object({
    private_ip = string
    public_ip  = string
  }))
  description = "The hosts to gather info from"
}

variable "expected_arch" {
  type = string
}

variable "expected_distro" {
  type = string
}

variable "expected_distro_version" {
  type = string
}

resource "enos_host_info" "hosts" {
  for_each = var.hosts

  transport = {
    ssh = {
      host = each.value.public_ip
    }
  }
}

output "results" {
  value = resource.enos_host_info.hosts
}

output "got_arch" {
  value = resource.enos_host_info.hosts[0].arch

  precondition {
    condition     = resource.enos_host_info.hosts[0].arch == var.expected_arch
    error_message = "The host doesn't have the expected architecture"
  }
}

output "got_distro" {
  value = resource.enos_host_info.hosts[0].distro

  precondition {
    condition     = resource.enos_host_info.hosts[0].distro == var.expected_distro
    error_message = "The host doesn't have the expected distro"
  }
}

output "got_distro_version" {
  value = resource.enos_host_info.hosts[0].distro_version

  precondition {
    condition     = resource.enos_host_info.hosts[0].distro_version == var.expected_distro_version
    error_message = "The host doesn't have the expected distro version"
  }
}
