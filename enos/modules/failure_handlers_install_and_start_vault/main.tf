# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}

locals {
  vault_release = {
    product = "vault"
    version = "1.12.0"
    edition = "ce"
  }

  vault_install_dir = "/opt/vault/bin"
  vault_bin_path    = "${local.vault_install_dir}/vault"
  vault_config_dir  = "/etc/vault.d"

  managed_service = true

  storage_backend = "inmem"
}

resource "enos_bundle_install" "vault" {
  destination = local.vault_install_dir
  release     = local.vault_release

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

resource "random_pet" "cluster_tag" {}

resource "enos_vault_start" "vault" {
  depends_on = [
    enos_bundle_install.vault,
  ]

  bin_path       = local.vault_bin_path
  config_dir     = local.vault_config_dir
  manage_service = local.managed_service

  config = {
    api_addr     = "http://${var.host_private_ip}:8200"
    cluster_addr = "http://${var.host_private_ip}:8201"
    cluster_name = random_pet.cluster_tag.id
    listener = {
      type = "tcp"
      attributes = {
        address     = "0.0.0.0:8200"
        tls_disable = "true"
      }
    }
    log_level = "debug"
    storage = {
      type       = local.storage_backend
      attributes = null
    }
    seal = {
      type       = "shamir"
      attributes = null
    }
    ui = true
  }
  unit_name = "vault"
  username  = "vault"

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}
