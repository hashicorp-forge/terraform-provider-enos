scenario "failure_handlers" {
  matrix {
    use = ["dev", "enos"]
  }

  locals {
    common_tags = {
      Name        = "enos-provider"
      Environment = var.environment
    }
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = terraform.default
  providers = [
    provider.aws.default,
    provider.enos.ubuntu,
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
      ami_architectures  = ["amd64"]
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

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip = step.setup_remote_host.public_ip
    }

    depends_on = [
      step.install_and_start_vault,
      step.install_and_start_consul
    ]
  }

  output "public_ip" {
    value = step.setup_remote_host.public_ip
  }
}
