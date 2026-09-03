# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# test_gather_logs_all_targets is the first half of the VAULT-42080 end-to-end test.
#
# It installs Vault and Consul, registers both SSH transports with the provider-level
# transportTargetRegistry, then intentionally fails a different resource (enos_remote_exec)
# to fire the failure handlers. Because debug_data_root_dir is configured on the provider,
# GatherLogsFromAllKnownTargetsFailureHandler writes log files for vault and consul to
# debug_data_root_dir on the local machine — even though those resources succeeded.
#
# This module is expected to fail (terraform apply exits non-zero). The verification that the
# log files were actually written is done in the separate test_gather_logs_all_targets_verify
# module, which runs in a subsequent enos step after this step's failure is handled by enos.

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
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

  consul_release = {
    product = "consul"
    version = "1.10.3"
    edition = "ce"
  }

  vault_install_dir  = "/opt/vault/bin"
  vault_bin_path     = "${local.vault_install_dir}/vault"
  vault_config_dir   = "/etc/vault.d"
  consul_install_dir = "/opt/consul/bin"
  consul_bin_path    = "${local.consul_install_dir}/consul"
  consul_data_dir    = "/opt/consul/data"
  consul_config_dir  = "/etc/consul.d"
}

resource "random_pet" "cluster_tag" {}

# ── Install Vault ────────────────────────────────────────────────────────────

resource "enos_bundle_install" "vault" {
  destination = local.vault_install_dir
  release     = local.vault_release

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

# ── Start Vault (registers SSH transport with the provider registry) ─────────

resource "enos_vault_start" "vault" {
  depends_on = [enos_bundle_install.vault]

  bin_path       = local.vault_bin_path
  config_dir     = local.vault_config_dir
  manage_service = true

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
      type       = "inmem"
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

# ── Install Consul ───────────────────────────────────────────────────────────

resource "enos_bundle_install" "consul" {
  destination = local.consul_install_dir
  release     = local.consul_release

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

# ── Start Consul (registers SSH transport with the provider registry) ────────

resource "enos_consul_start" "consul" {
  depends_on = [enos_bundle_install.consul]

  bin_path   = local.consul_bin_path
  data_dir   = local.consul_data_dir
  config_dir = local.consul_config_dir
  config = {
    datacenter       = "dc1"
    retry_join       = ["127.0.0.1"]
    data_dir         = local.consul_data_dir
    log_level        = "INFO"
    server           = true
    bootstrap_expect = 1
    log_file         = local.consul_config_dir
  }
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

# ── Intentionally failing resource ───────────────────────────────────────────
# This triggers the failure handlers. Because enos_vault_start and enos_consul_start
# both registered their SSH transport above, GatherLogsFromAllKnownTargetsFailureHandler
# will attempt to collect vault and consul logs and write them to debug_data_root_dir.

resource "enos_remote_exec" "trigger_failure" {
  depends_on = [
    enos_vault_start.vault,
    enos_consul_start.consul,
  ]

  # This command does not exist — it is intentionally designed to fail.
  inline = ["this-command-does-not-exist"]

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

