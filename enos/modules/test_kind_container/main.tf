# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

variable "kubeconfig_base64" {
  type        = string
  description = "Base64 encoded kubeconfig for the kind cluster"
}

variable "context_name" {
  type        = string
  description = "The context to connect to"
}

variable "pod_label_selectors" {
  type        = list(string)
  description = "Lable selectors to use when querying the pods to test"
}

variable "replica_count" {
  type        = number
  description = "The expected number of pods that were created"
}

variable "namespace" {
  description = "The namespace to the pods where deployed in"
  type        = string
}

data "enos_kubernetes_pods" "ci_test_pods" {
  kubeconfig_base64 = var.kubeconfig_base64
  context_name      = var.context_name
  label_selectors   = var.pod_label_selectors
  namespace         = var.namespace
}

locals {
  pod_names      = [for pod in data.enos_kubernetes_pods.ci_test_pods.pods : pod.name]
  pod_namespaces = [for pod in data.enos_kubernetes_pods.ci_test_pods.pods : pod.namespace]
}

resource "enos_remote_exec" "test_container" {
  count  = var.replica_count
  inline = ["touch /tmp/its_alive"]

  transport = {
    kubernetes = {
      kubeconfig_base64 = var.kubeconfig_base64
      context_name      = var.context_name
      namespace         = local.pod_namespaces[count.index]
      pod               = local.pod_names[count.index]
    }
  }
}

output "pods" {
  value = data.enos_kubernetes_pods.ci_test_pods.pods
}
