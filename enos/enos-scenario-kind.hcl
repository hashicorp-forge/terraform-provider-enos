# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

scenario "kind" {
  matrix {
    use = ["dev", "prod"]
  }

  locals {
    pod_replica_count = 3
    helm_provider = {
      "dev"  = provider.helm.kind_dev
      "prod" = provider.helm.kind_prod
    }
    kubeconfig_path = abspath(joinpath(path.root, "kubeconfig_kind_${matrix.use}"))
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = terraform.k8s
  providers = [
    provider.enos.default,
    provider.helm.kind_dev,
    provider.helm.kind_prod,
  ]

  step "create_kind_cluster" {
    module = module.kind_create_test_cluster

    variables {
      kubeconfig_path = local.kubeconfig_path
    }
  }

  step "deploy_helm_chart" {
    module = module.helm_chart

    providers = {
      helm = local.helm_provider[matrix.use]
    }

    variables {
      host                   = step.create_kind_cluster.host
      client_certificate     = step.create_kind_cluster.client_certificate
      client_key             = step.create_kind_cluster.client_key
      cluster_ca_certificate = step.create_kind_cluster.cluster_ca_certificate
      replica_count          = local.pod_replica_count
      repository             = step.create_kind_cluster.repository
      tag                    = step.create_kind_cluster.tag
    }
  }

  step "test_container" {
    depends_on = [
      step.deploy_helm_chart,
    ]
    module = module.test_kind_container

    variables {
      kubeconfig_base64 = step.create_kind_cluster.kubeconfig_base64
      context_name      = step.create_kind_cluster.context_name
      replica_count     = local.pod_replica_count
      namespace         = step.deploy_helm_chart.namespace
      pod_label_selectors = [
        "app.kubernetes.io/instance=ci-test",
        "app.kubernetes.io/name=ci-test"
      ]
    }
  }

  output "cluster_name" {
    value = step.create_kind_cluster.cluster_name
  }

  output "pods_tested" {
    value = step.test_container.pods
  }
}
