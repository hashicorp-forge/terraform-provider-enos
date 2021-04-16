terraform {
  required_providers {
    enos = {
      version = "~> 0.1.0"
      source  = "hashicorp.com/qti/enos"
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
