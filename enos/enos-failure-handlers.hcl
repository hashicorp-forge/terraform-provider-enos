terraform_cli "default" {
  provider_installation {
    filesystem_mirror {
      path    = abspath(joinpath(path.root, "../dist"))
      include = ["app.terraform.io/*/*"]
    }
    direct {
      exclude = ["app.terraform.io/*/*"]
    }
  }
}

terraform "default" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }

    aws = {
      source = "hashicorp/aws"
    }

    random = {
      source = "hashicorp/random"
    }
  }
}

module "setup_remote_host" {
  source = abspath("./modules/setup_remote_host")
}

module "install_and_start_vault" {
  source = abspath("./modules/install_and_start_vault")
}

module "install_and_start_consul" {
  source = abspath("./modules/install_and_start_consul")
}

module "test_failure_handlers" {
  source = abspath("./modules/test_failure_handlers")
}

module "create_vpc" {
  source = abspath("./modules/create_vpc")
}

variable "run_failure_handler_tests" {
  description = "Whether or not to run the failure handlers tests"
  type        = bool
  default     = false
}

variable "environment" {
  description = "The environment that the scenario is being run in"
  type        = string
  default     = "ci"
}

scenario "failure_handlers" {

  locals {
    common_tags = {
      Name        = "enos_provider_remote_host"
      Environment = var.environment
    }
    instance_type = "t3.micro"
  }

  terraform_cli = terraform_cli.default
  terraform     = terraform.default
  providers = [
    provider.aws.east,
    provider.enos.ubuntu,
  ]

  step "create_vpc" {
    module = module.create_vpc

    providers = {
      aws  = provider.aws.east
    }

    variables {
      tags          = local.common_tags
      instance_type = local.instance_type
    }
  }

  step "setup_remote_host" {
    module = module.setup_remote_host

    providers = {
      aws  = provider.aws.east
      enos = provider.enos.ubuntu
    }

    variables {
      vpc_id        = step.create_vpc.vpc_id
      subnet_id     = step.create_vpc.subnet_id
      tags          = local.common_tags
      instance_type = local.instance_type
    }

    depends_on = [step.create_vpc]
  }

  step "install_and_start_vault" {
    module = module.install_and_start_vault

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      host_public_ip  = step.setup_remote_host.public_ip
      host_private_ip = step.setup_remote_host.private_ip
    }
  }

  step "install_and_start_consul" {
    module = module.install_and_start_consul

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
