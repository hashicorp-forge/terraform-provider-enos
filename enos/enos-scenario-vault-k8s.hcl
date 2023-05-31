scenario "vault_k8s" {
  matrix {
    edition = ["oss", "ent"]
    use     = ["dev", "enos", "enosdev"]
  }

  locals {
    image_repo = var.image_repository != null ? var.image_repository : matrix.edition == "oss" ? "hashicorp/vault" : "hashicorp/vault-enterprise"
    helm_provider = {
      "oss" = {
        "dev"     = provider.helm.oss_dev
        "enos"    = provider.helm.oss_enos
        "enosdev" = provider.helm.oss_enosdev
      }
      "ent" = {
        "dev"     = provider.helm.ent_dev
        "enos"    = provider.helm.ent_enos
        "enosdev" = provider.helm.ent_enosdev
      }
    }
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = matrix.use == "enosdev" ? terraform.k8s_enosdev : terraform.k8s
  providers = [
    provider.enos.default,
    provider.helm.oss_dev,
    provider.helm.oss_enos,
    provider.helm.oss_enosdev,
    provider.helm.ent_dev,
    provider.helm.ent_enos,
    provider.helm.ent_enosdev,
  ]

  step "read_license" {
    skip_step = matrix.edition == "oss"
    module    = module.read_file

    variables {
      file_name = abspath(joinpath(path.root, "./support/vault.hclic"))
    }
  }

  step "create_kind_cluster" {
    module = matrix.use == "enosdev" ? module.create_kind_cluster_enosdev : module.create_kind_cluster

    variables {
      kubeconfig_path = abspath(joinpath(path.root, "kubeconfig_${matrix.edition}_${matrix.use}"))
    }
  }

  step "deploy_vault" {
    depends_on = [
      step.create_kind_cluster,
    ]
    module = matrix.use == "enosdev" ? module.k8s_deploy_vault_enosdev : module.k8s_deploy_vault

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
      ent_license       = matrix.edition != "oss" ? step.read_license.license : null
    }
  }

  step "verify_write_data" {
    depends_on = [
      step.deploy_vault,
    ]
    module = matrix.use == "enosdev" ? module.k8s_verify_write_data_enosdev : module.k8s_verify_write_data

    variables {
      vault_pods        = step.deploy_vault.vault_pods
      vault_root_token  = step.deploy_vault.vault_root_token
      kubeconfig_base64 = step.create_kind_cluster.kubeconfig_base64
      context_name      = step.create_kind_cluster.context_name
    }
  }
}
