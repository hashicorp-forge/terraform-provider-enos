scenario "k8s" {
  matrix {
    edition = ["oss", "ent"]
  }

  terraform_cli = terraform_cli.default
  terraform     = terraform.k8s

  providers = [
    provider.enos.default,
    provider.helm.default,
  ]

  locals {
    image_repo = var.image_repository != null ? var.image_repository : matrix.edition == "oss" ? "hashicorp/vault" : "hashicorp/vault-enterprise"
  }

  step "read_license" {
    skip_step = matrix.edition == "oss"
    module    = module.read_license

    variables {
      file_name = abspath(joinpath(path.root, "./support/vault.hclic"))
    }
  }

  step "create_kind_cluster" {
    module = module.create_kind_cluster

    variables {
      kubeconfig_path = abspath(joinpath(path.root, "kubeconfig"))
    }
  }

  step "deploy_vault" {
    depends_on = [
      step.create_kind_cluster,
    ]
    module = module.k8s_deploy_vault

    variables {
      image_tag         = var.image_tag
      context_name      = step.create_kind_cluster.context_name
      image_repository  = local.image_repo
      kubeconfig_base64 = step.create_kind_cluster.kubeconfig_base64
      vault_edition     = matrix.edition
      vault_log_level   = var.log_level
      ent_license       = matrix.edition != "oss" ? step.read_license.license : null
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
