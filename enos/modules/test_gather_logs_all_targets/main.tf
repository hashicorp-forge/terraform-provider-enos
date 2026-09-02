# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# test_gather_logs_all_targets verifies VAULT-42080: when any resource fails, the provider's
# failure handlers gather logs from *all* registered transport targets — not only the one that
# failed.
#
# How it works:
#   1. enos_bundle_install + enos_vault_start + enos_consul_start all apply successfully, each
#      registering their SSH transport with the provider-level transportTargetRegistry.
#   2. enos_remote_exec.trigger_failure intentionally runs a command that will never exist,
#      causing the apply to fail and firing every registered failure handler.
#   3. Because debug_data_root_dir is set on the provider, GatherLogsFromAllKnownTargetsFailureHandler
#      writes log files for vault and consul to disk — even though those resources succeeded.
#   4. enos_remote_exec.verify_log_files then SSHes into the host and checks that the expected
#      log files exist locally (via a path passed as an env var), proving that logs were gathered
#      from non-failing targets.
#
# NOTE: step 2 is intentionally expected to fail. The verification in step 4 is what confirms
# the gather-all-targets behaviour worked correctly.

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

# ── Verify log files were written by the gather-all-targets handler ──────────
# After the failure above, the provider should have written log files for vault
# and consul to debug_data_root_dir on the local machine running terraform.
# We verify those files exist using a local-exec so we stay in the same apply.

resource "enos_local_exec" "verify_vault_log_written" {
  depends_on = [enos_remote_exec.trigger_failure]

  # The vault systemd journal log file follows the naming convention:
  # <host>_vault.log — verify it exists and is non-empty.
  inline = [
    "set -e",
    "log_dir=${var.debug_data_root_dir}",
    "echo 'Checking for vault log file in: '\"$log_dir\"",
    "vault_log=$(ls \"$log_dir\"/*vault* 2>/dev/null | head -1)",
    "if [ -z \"$vault_log\" ]; then echo 'ERROR: no vault log file found in '\"$log_dir\"; exit 1; fi",
    "echo 'Found vault log: '\"$vault_log\"",
    "if [ ! -s \"$vault_log\" ]; then echo 'ERROR: vault log file is empty'; exit 1; fi",
    "echo 'vault log file verified OK'",
  ]
}

resource "enos_local_exec" "verify_consul_log_written" {
  depends_on = [enos_remote_exec.trigger_failure]

  inline = [
    "set -e",
    "log_dir=${var.debug_data_root_dir}",
    "echo 'Checking for consul log file in: '\"$log_dir\"",
    "consul_log=$(ls \"$log_dir\"/*consul* 2>/dev/null | head -1)",
    "if [ -z \"$consul_log\" ]; then echo 'ERROR: no consul log file found in '\"$log_dir\"; exit 1; fi",
    "echo 'Found consul log: '\"$consul_log\"",
    "if [ ! -s \"$consul_log\" ]; then echo 'ERROR: consul log file is empty'; exit 1; fi",
    "echo 'consul log file verified OK'",
  ]
}
