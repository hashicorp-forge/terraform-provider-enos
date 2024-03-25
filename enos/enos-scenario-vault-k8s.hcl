# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

scenario "vault_k8s" {
  matrix {
    edition = ["ce", "ent"]
    use     = ["dev", "prod"]
  }

  locals {
    image_repo = var.image_repository != null ? var.image_repository : matrix.edition == "ce" ? "hashicorp/vault" : "hashicorp/vault-enterprise"
    helm_provider = {
      "ce" = {
        "dev"  = provider.helm.ce_dev
        "prod" = provider.helm.ce_prod
      }
      "ent" = {
        "dev"  = provider.helm.ent_dev
        "prod" = provider.helm.ent_prod
      }
    }
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = terraform.k8s
  providers = [
    provider.enos.default,
    provider.helm.ce_dev,
    provider.helm.ce_prod,
    provider.helm.ent_dev,
    provider.helm.ent_prod,
  ]

  step "read_license" {
    skip_step = matrix.edition == "ce"
    module    = module.read_file

    variables {
      file_name = abspath(joinpath(path.root, "./support/vault.hclic"))
    }
  }

  step "create_kind_cluster" {
    module = module.create_kind_cluster

    variables {
      kubeconfig_path = abspath(joinpath(path.root, "kubeconfig_${matrix.edition}_${matrix.use}"))
    }
  }

  step "deploy_vault" {
    depends_on = [
      step.create_kind_cluster,
    ]
    module = module.k8s_deploy_vault

    providers = {
      helm = local.helm_provider[matrix.edition][matrix.use]
    }

    variables {
      image_tag         = var.image_tag
      context_name      = step.create_kind_cluster.context_name
      image_repository  = local.image_repo
      kubeconfig_base64 = step.create_kind_cluster.kubeconfig_base64
      vault_edition     = matrix.edition
      vault_log_level   = var.log_level
      ent_license       = matrix.edition != "ce" ? step.read_license.content : null
    }
  }

  step "verify_write_data" {
    depends_on = [
      step.deploy_vault,
    ]
    module = module.k8s_verify_write_data

    variables {
      vault_pods        = step.deploy_vault.vault_pods
      vault_root_token  = step.deploy_vault.vault_root_token
      kubeconfig_base64 = step.create_kind_cluster.kubeconfig_base64
      context_name      = step.create_kind_cluster.context_name
    }
  }
}
