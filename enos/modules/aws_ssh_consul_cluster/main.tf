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

data "enos_environment" "localhost" {}

locals {
  name_suffix        = "${var.project_name}-${var.environment}"
  consul_bin_path    = "${var.consul_install_dir}/consul"
  consul_cluster_tag = "consul-server-${random_string.cluster_id.result}"
}

resource "random_string" "cluster_id" {
  length  = 8
  lower   = true
  upper   = false
  numeric = false
  special = false
}
