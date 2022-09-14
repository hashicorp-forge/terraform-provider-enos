variable "tags" {
  description = "Tags to add to AWS resources"
  type        = map(string)
  default     = null
}

terraform_cli "default" {
  credentials "app.terraform.io" {
    token = var.tfc_api_token
  }
}

terraform "default" {
  required_version = ">= 1.0.0"

  required_providers {
    enos = {
      source  = joinpath("app.terraform.io/hashicorp-qti", var.enos_provider_name)
      version = var.enos_provider_version
    }

    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "aws" "east" {
  region = "us-east-1"
}

provider "enos" "ubuntu" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = abspath(joinpath(path.root, "./support/enos-ci-ssh-key.pem"))
    }
  }
}

provider "enos" "rhel" {
  transport = {
    ssh = {
      user             = "ec2_user"
      private_key_path = abspath(joinpath(path.root, "./support/enos-ci-ssh-key.pem"))
    }
  }
}

module "enos_infra" {
  source = "./modules/infra"

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags  = var.tags
}

module "backend_raft" {
  source = "./modules/raft_cluster"
}

module "backend_consul" {
  source = "./modules/backend_consul"

  project_name    = "qti-enos-provider"
  environment     = "ci"
  common_tags     = var.tags
  ssh_aws_keypair = "enos-ci-ssh-key"

  consul_license = "none"
}

module "vault_cluster" {
  source = "./modules/vault_cluster"

  project_name    = "qti-enos-provider"
  environment     = "ci"
  common_tags     = var.tags
  ssh_aws_keypair = "enos-ci-ssh-key"
}

module "license" {
  source = "./modules/license"
}

scenario "vault" {
  matrix {
    backend = ["consul", "raft"]
    distro  = ["ubuntu", "rhel"]
    arch    = ["amd64", "arm64"]
    edition = ["oss", "ent"]
  }

  locals {
    enos_provider = {
      rhel   = provider.enos.rhel
      ubuntu = provider.enos.ubuntu
    }
    enos_transport_user = {
      rhel   = "ec2-user"
      ubuntu = "ubuntu"
    }
    vault_version = "1.10.2"
  }

  terraform_cli = terraform_cli.default
  terraform     = terraform.default
  providers = [
    provider.aws.east,
    provider.enos.ubuntu,
    provider.enos.rhel
  ]

  step "infra" {
    module = module.enos_infra
    variables {
      ami_architectures = ["amd64", "arm64"]
    }
  }

  step "license" {
    module = module.license
    variables {
      file_name = abspath(joinpath(path.root, "./support/vault.hclic"))
    }
  }

  step "backend" {
    skip_step  = matrix.backend == "raft"
    depends_on = [step.infra]
    providers = {
      enos = provider.enos.ubuntu
    }
    module = "backend_consul"
    variables {
      ami_id      = step.infra.ami_ids["ubuntu"]["amd64"]
      vpc_id      = step.infra.vpc_id
      kms_key_arn = step.infra.kms_key_arn
    }
  }

  step "vault_with_consul" {
    skip_step  = matrix.backend == "raft"
    module     = module.vault_cluster
    depends_on = [step.backend]
    providers = {
      enos = local.enos_provider[matrix.distro]
    }
    variables {
      ami_id              = step.infra.ami_ids[matrix.distro][matrix.arch]
      vpc_id              = step.infra.vpc_id
      kms_key_arn         = step.infra.kms_key_arn
      storage_backend     = matrix.backend
      enos_transport_user = local.enos_transport_user[matrix.distro]
      consul_cluster_tag  = step.backend.consul_cluster_tag
      vault_release = {
        version = local.vault_version
        edition = matrix.edition
      }
      vault_license = matrix.edition != "oss" ? semverconstraint(local.vault_version, "> 1.8.0") ? step.license.license : null : null
    }
  }

  step "vault_with_raft" {
    skip_step  = matrix.backend == "consul"
    module     = module.vault_cluster
    depends_on = [step.infra]
    providers = {
      enos = local.enos_provider[matrix.distro]
    }
    variables {
      ami_id              = step.infra.ami_ids[matrix.distro][matrix.arch]
      vpc_id              = step.infra.vpc_id
      kms_key_arn         = step.infra.kms_key_arn
      storage_backend     = "raft"
      enos_transport_user = local.enos_transport_user[matrix.distro]
      consul_cluster_tag  = null
      vault_release = {
        version = local.vault_version
        edition = matrix.edition
      }
      vault_license = matrix.edition != "oss" ? semverconstraint(local.vault_version, "> 1.8.0") ? step.license.license : null : null
    }
  }

  output "vault_cluster_instance_ids" {
    description = "The Vault cluster instance IDs"
    value       = matrix.backend == "consul" ? step.vault_with_consul.instance_ids : step.vault_with_raft.instance_ids
  }

  output "vault_cluster_pub_ips" {
    description = "The Vault cluster public IPs"
    value       = matrix.backend == "consul" ? step.vault_with_consul.instance_public_ips : step.vault_with_raft.instance_public_ips
  }

  output "vault_cluster_priv_ips" {
    description = "The Vault cluster private IPs"
    value       = matrix.backend == "consul" ? step.vault_with_consul.instance_private_ips : step.vault_with_raft.instance_private_ips
  }

  output "vault_cluster_key_id" {
    description = "The Vault cluster Key ID"
    value       = matrix.backend == "consul" ? step.vault_with_consul.key_id : step.vault_with_raft.key_id
  }

  output "vault_cluster_root_token" {
    description = "The Vault cluster root token"
    value       = matrix.backend == "consul" ? step.vault_with_consul.vault_root_token : step.vault_with_raft.vault_root_token
  }

  output "vault_cluster_unseal_keys_b64" {
    description = "The Vault cluster unseal keys"
    value       = matrix.backend == "consul" ? step.vault_with_consul.vault_unseal_keys_b64 : step.vault_with_raft.vault_unseal_keys_b64
  }

  output "vault_cluster_unseal_keys_hex" {
    description = "The Vault cluster unseal keys hex"
    value       = matrix.backend == "consul" ? step.vault_with_consul.vault_unseal_keys_hex : step.vault_with_raft.vault_unseal_keys_hex
  }

  output "vault_cluster_tag" {
    description = "The Vault cluster tag"
    value       = matrix.backend == "consul" ? step.vault_with_consul.vault_cluster_tag : step.vault_with_raft.vault_cluster_tag
  }
}
