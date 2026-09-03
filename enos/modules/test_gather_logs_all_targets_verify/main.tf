# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# test_gather_logs_all_targets_verify is the second half of the VAULT-42080 end-to-end test.
#
# It runs after test_gather_logs_all_targets (which is expected to fail). By the time this
# module is applied, the failure handlers should have written vault and consul log files to
# debug_data_root_dir on the local machine. This module asserts those files exist and are
# non-empty, confirming that GatherLogsFromAllKnownTargetsFailureHandler collected logs from
# non-failing registered targets.

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

resource "enos_local_exec" "verify_vault_log_written" {
  # The vault systemd journal log file written by the failure handler follows the naming
  # convention: <host>_vault.log (or similar). We glob for any file matching *vault* so
  # we are not sensitive to the exact host address used in the filename.
  inline = [
    "set -e",
    "log_dir='${var.debug_data_root_dir}'",
    "echo \"Checking for vault log file in: $log_dir\"",
    "vault_log=$(ls \"$log_dir\"/*vault* 2>/dev/null | head -1)",
    "if [ -z \"$vault_log\" ]; then echo \"ERROR: no vault log file found in $log_dir\"; exit 1; fi",
    "echo \"Found vault log: $vault_log\"",
    "if [ ! -s \"$vault_log\" ]; then echo 'ERROR: vault log file is empty'; exit 1; fi",
    "echo 'vault log file verified OK'",
  ]
}

resource "enos_local_exec" "verify_consul_log_written" {
  inline = [
    "set -e",
    "log_dir='${var.debug_data_root_dir}'",
    "echo \"Checking for consul log file in: $log_dir\"",
    "consul_log=$(ls \"$log_dir\"/*consul* 2>/dev/null | head -1)",
    "if [ -z \"$consul_log\" ]; then echo \"ERROR: no consul log file found in $log_dir\"; exit 1; fi",
    "echo \"Found consul log: $consul_log\"",
    "if [ ! -s \"$consul_log\" ]; then echo 'ERROR: consul log file is empty'; exit 1; fi",
    "echo 'consul log file verified OK'",
  ]
}
