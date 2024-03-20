# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_providers {
    enos = {
      version = ">= 0.3"
      source  = "app.terraform.io/hashicorp-qti/enos"
    }
  }
}

provider "enos" {
}

data "enos_artifactory_item" "vault" {
  username   = var.artifactory_username
  token      = var.artifactory_token
  host       = var.artifactory_host
  repo       = var.artifactory_repo
  path       = var.artifactory_path
  name       = var.artifactory_name
  properties = length(var.artifactory_properties) > 0 ? var.artifactory_properties : null
}

// Make sure you export ENOS_TRANSPORT_USER, ENOS_TRANSPORT_PRIVATE_KEY_PATH, and ENOS_TRANSPORT_HOST
// so that we can reach the target
resource "enos_bundle_install" "vault_from_artifactory" {
  depends_on = [data.enos_artifactory_item.vault]

  destination = "/opt/vault/bin"

  artifactory = {
    username = var.artifactory_username
    token    = var.artifactory_token
    url      = data.enos_artifactory_item.vault.results[0].url
    sha256   = data.enos_artifactory_item.vault.results[0].sha256
  }
}

resource "enos_bundle_install" "boundary_from_releases" {
  depends_on = [enos_bundle_install.vault_from_artifactory]

  destination = "/opt/boundary/bin"

  release = {
    edition = "oss"
    version = "0.1.0"
    product = "boundary"
  }
}

resource "enos_bundle_install" "fixture_from_local_path" {
  depends_on = [enos_bundle_install.boundary_from_releases]

  path        = "../../internal/fixtures/bundle.zip"
  destination = "/opt/fixture/bin"
}
