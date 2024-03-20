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

resource "enos_remote_exec" "should_fail" {
  inline = ["eat barf"]

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}
