# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

scenario "failure_handlers" {
  matrix {
    use = ["dev", "enos"]
  }

  locals {
    common_tags = {
      Name        = "enos-provider"
      Environment = var.environment
    }

    # Local directory where the ubuntu_with_debug provider writes failure-handler log files.
    # This must match the debug_data_root_dir configured on provider.enos.ubuntu_with_debug.
    debug_data_root_dir = abspath(joinpath(path.root, "../.enos-debug-logs"))
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = terraform.default
  providers = [
    provider.aws.default,
    provider.enos.ubuntu,
    provider.enos.ubuntu_with_debug,
  ]

  step "find_azs" {
    module = module.az_finder

    variables {
      instance_type = ["t3.micro"]
    }
  }

  step "create_vpc" {
    module = module.aws_infra

    variables {
      availability_zones = step.find_azs.availability_zones
    }
  }

  step "setup_remote_host" {
    module = module.failure_handlers_setup_remote_host

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      vpc_id        = step.create_vpc.vpc_id
      tags          = local.common_tags
      instance_type = "t3.micro"
    }

    depends_on = [step.create_vpc]
  }

  step "install_and_start_vault" {
    module = module.failure_handlers_install_and_start_vault

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip  = step.setup_remote_host.public_ip
      host_private_ip = step.setup_remote_host.private_ip
    }
  }

  step "install_and_start_consul" {
    module = module.failure_handlers_install_and_start_consul

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip = step.setup_remote_host.public_ip
    }
  }

  step "test_failure_handlers" {
    skip_step = !var.run_failure_handler_tests
    module    = module.test_failure_handlers
    depends_on = [
      step.install_and_start_vault,
      step.install_and_start_consul
    ]

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip = step.setup_remote_host.public_ip
    }
  }

  # test_gather_logs_all_targets verifies VAULT-42080.
  # It installs Vault + Consul, then intentionally fails a different resource and asserts
  # that log files for *both* vault and consul were written to disk by the
  # GatherLogsFromAllKnownTargetsFailureHandler — even though those resources succeeded.
  #
  # The step uses provider.enos.ubuntu_with_debug so that debug_data_root_dir is set and
  # log files are actually persisted. skip_step mirrors run_failure_handler_tests so the
  # end-to-end test is opt-in (same behaviour as the existing test_failure_handlers step).
  step "test_gather_logs_all_targets" {
    skip_step  = !var.run_failure_handler_tests
    module     = module.test_gather_logs_all_targets
    depends_on = [step.setup_remote_host]

    providers = {
      enos = provider.enos.ubuntu_with_debug
    }

    variables {
      host_public_ip      = step.setup_remote_host.public_ip
      host_private_ip     = step.setup_remote_host.private_ip
      debug_data_root_dir = local.debug_data_root_dir
    }
  }

  step "test_enos_user" {
    depends_on = [
      step.setup_remote_host
    ]
    module = module.test_enos_user

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip = step.setup_remote_host.public_ip
    }
  }

  output "public_ip" {
    value = step.setup_remote_host.public_ip
  }
}
