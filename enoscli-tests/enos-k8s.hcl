
terraform_cli "k8s" {
  credentials "app.terraform.io" {
    token = "TFC_API_TOKEN"
  }
}

terraform "k8s" {
  required_version = ">= 1.0.0"

  required_providers {
    enos = {
      source  = "app.terraform.io/hashicorp-qti/ENOS_RELEASE_NAME"
      version = ">= 0.2.1"
    }
  }
}

provider "enos" "k8s" {}

module "kind_cluster" {
  source = "./modules/kind-cluster"
}

module "helm_chart" {
  source = "./modules/deploy-helm-chart"
}

module "test_container" {
  source = "./modules/test-container"
}

scenario "kind_cluster" {

  providers = [provider.enos.k8s]

  terraform     = terraform.k8s
  terraform_cli = terraform_cli.k8s

  locals {
    pod_replica_count = 3
  }

  step "create_cluster" {
    module = module.kind_cluster

    providers = {
      enos = provider.enos.k8s
    }
  }

  step "deploy_helm_chart" {
    module = module.helm_chart

    variables {
      host                   = step.create_cluster.host
      client_certificate     = step.create_cluster.client_certificate
      client_key             = step.create_cluster.client_key
      cluster_ca_certificate = step.create_cluster.cluster_ca_certificate
      replica_count          = local.pod_replica_count
    }
  }

  step "test_container" {
    module = module.test_container

    providers = {
      enos = provider.enos.k8s
    }

    variables {
      kubeconfig_base64 = step.create_cluster.kubeconfig_base64
      context_name      = step.create_cluster.context_name
      replica_count     = local.pod_replica_count
      pod_label_selectors = [
        "app.kubernetes.io/instance=ci-test",
        "app.kubernetes.io/name=ci-test"
      ]
    }

    depends_on = [step.deploy_helm_chart]
  }

  output "cluster_name" {
    value = step.create_cluster.cluster_name
  }

  output "pods_tested" {
    value = step.test_container.pods
  }
}
