# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }
  }
}

variable "kubeconfig_path" {
  type = string
}

locals {
  image_name = "bananas"
  image_tag  = "0.1.0"
}

resource "random_pet" "cluster_name" {}

resource "enos_local_kind_cluster" "test" {
  name            = random_pet.cluster_name.id
  kubeconfig_path = var.kubeconfig_path
}

resource "enos_local_exec" "create_bananas" {
  environment = {
    IMAGE_NAME = local.image_name
    IMAGE_TAG  = local.image_tag
  }
  inherit_environment = true
  scripts             = [abspath("${path.module}/scripts/image.sh")]
}

resource "enos_local_kind_load_image" "bananas" {
  cluster_name = random_pet.cluster_name.id
  image        = local.image_name
  tag          = local.image_tag

  depends_on = [enos_local_kind_cluster.test, enos_local_exec.create_bananas]
}

output "cluster_name" {
  value = random_pet.cluster_name.id
}

output "kubeconfig_path" {
  value = var.kubeconfig_path
}

output "kubeconfig_base64" {
  value = enos_local_kind_cluster.test.kubeconfig_base64
}

output "context_name" {
  value = enos_local_kind_cluster.test.context_name
}

output "host" {
  value = enos_local_kind_cluster.test.endpoint
}

output "client_certificate" {
  value = enos_local_kind_cluster.test.client_certificate
}

output "client_key" {
  value = enos_local_kind_cluster.test.client_key
}

output "cluster_ca_certificate" {
  value = enos_local_kind_cluster.test.cluster_ca_certificate
}

output "repository" {
  value = enos_local_kind_load_image.bananas.loaded_images.repository
}

output "tag" {
  value = enos_local_kind_load_image.bananas.loaded_images.tag
}
