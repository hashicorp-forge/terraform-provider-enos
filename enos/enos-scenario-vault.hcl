scenario "vault" {
  matrix {
    backend       = ["consul", "raft"]
    distro        = ["ubuntu", "rhel"]
    arch          = ["amd64", "arm64"]
    edition       = ["oss", "ent"]
    version       = ["1.8.12", "1.9.10", "1.10.11", "1.11.10", "1.12.6", "1.13.2"]
    unseal_method = ["shamir", "awskms"]
    use           = ["dev", "enos", "enosdev"]
  }

  locals {
    enos_provider = {
      rhel   = provider.enos.rhel
      ubuntu = provider.enos.ubuntu
    }
    instance_types = {
      amd64 = "t3a.small"
      arm64 = "t4g.small"
    }
    vault_version = matrix.version
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = matrix.use == "enosdev" ? "enosdev" : "default"
  providers = [
    provider.aws.default,
    provider.enos.ubuntu,
    provider.enos.rhel,
  ]

  step "find_azs" {
    module = module.az_finder

    variables {
      instance_type = values(local.instance_types)
    }
  }

  step "infra" {
    module = module.aws_infra

    variables {
      ami_architectures  = ["amd64", "arm64"]
      availability_zones = step.find_azs.availability_zones
    }
  }

  step "license" {
    skip_step = matrix.edition == "oss"
    module    = module.read_file

    variables {
      file_name = abspath(joinpath(path.root, "./support/vault.hclic"))
    }
  }

  step "backend" {
    skip_step = matrix.backend == "raft"
    depends_on = [
      step.infra,
    ]
    module = matrix.use == "enosdev" ? module.aws_ssh_consul_cluster_enosdev : module.aws_ssh_consul_cluster

    providers = {
      enos = provider.enos.ubuntu
    }

    variables {
      ami_id        = step.infra.ami_ids["ubuntu"]["amd64"]
      vpc_id        = step.infra.vpc_id
      instance_type = local.instance_types["amd64"]
      kms_key_arn   = step.infra.kms_key_arn
    }
  }

  step "vault_with_consul" {
    skip_step = matrix.backend == "raft"
    depends_on = [
      step.backend,
    ]
    module = matrix.use == "enosdev" ? module.aws_ssh_vault_cluster_enosdev : module.aws_ssh_vault_cluster

    providers = {
      enos = local.enos_provider[matrix.distro]
    }

    variables {
      ami_id             = step.infra.ami_ids[matrix.distro][matrix.arch]
      consul_cluster_tag = step.backend.consul_cluster_tag
      instance_type      = local.instance_types[matrix.arch]
      kms_key_arn        = step.infra.kms_key_arn
      storage_backend    = matrix.backend
      unseal_method      = matrix.unseal_method
      vault_license      = matrix.edition != "oss" ? step.license.license : null
      vault_release = {
        version = local.vault_version
        edition = matrix.edition
      }
      vpc_id = step.infra.vpc_id
    }
  }

  step "vault_with_raft" {
    skip_step = matrix.backend == "consul"
    depends_on = [
      step.infra,
    ]
    module = matrix.use == "enosdev" ? module.aws_ssh_vault_cluster_enosdev : module.aws_ssh_vault_cluster

    providers = {
      enos = local.enos_provider[matrix.distro]
    }

    variables {
      ami_id             = step.infra.ami_ids[matrix.distro][matrix.arch]
      consul_cluster_tag = null
      instance_type      = local.instance_types[matrix.arch]
      kms_key_arn        = step.infra.kms_key_arn
      storage_backend    = "raft"
      unseal_method      = matrix.unseal_method
      vault_license      = matrix.edition != "oss" ? step.license.license : null
      vault_release = {
        version = local.vault_version
        edition = matrix.edition
      }
      vpc_id = step.infra.vpc_id
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
