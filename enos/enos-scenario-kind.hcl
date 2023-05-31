scenario "kind" {
  matrix {
    use = ["dev", "enos", "enosdev"]
  }

  locals {
    pod_replica_count = 3
  }

  terraform_cli = matrix.use == "dev" ? terraform_cli.dev : terraform_cli.default
  terraform     = matrix.use == "enosdev" ? terraform.k8s_enosdev : terraform.k8s
  providers = [
    provider.enos.default,
    provider.helm.default,
  ]

  step "create_cluster" {
    module = matrix.use == "enosdev" ? module.kind_create_test_cluster_enosdev : module.kind_create_test_cluster
  }

  step "deploy_helm_chart" {
    module = module.helm_chart

    variables {
      host                   = step.create_cluster.host
      client_certificate     = step.create_cluster.client_certificate
      client_key             = step.create_cluster.client_key
      cluster_ca_certificate = step.create_cluster.cluster_ca_certificate
      replica_count          = local.pod_replica_count
      repository             = step.create_cluster.repository
      tag                    = step.create_cluster.tag
    }
  }

  step "test_container" {
    depends_on = [
      step.deploy_helm_chart,
    ]
    module = matrix.use == "enosdev" ? module.test_kind_container_enosdev : module.test_kind_container

    variables {
      kubeconfig_base64 = step.create_cluster.kubeconfig_base64
      context_name      = step.create_cluster.context_name
      replica_count     = local.pod_replica_count
      namespace         = step.deploy_helm_chart.namespace
      pod_label_selectors = [
        "app.kubernetes.io/instance=ci-test",
        "app.kubernetes.io/name=ci-test"
      ]
    }
  }

  output "cluster_name" {
    value = step.create_cluster.cluster_name
  }

  output "pods_tested" {
    value = step.test_container.pods
  }
}
