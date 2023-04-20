terraform {
  required_providers {
    enos = {
      source  = "app.terraform.io/hashicorp-qti/ENOS_PROVIDER_NAME"
      version = "ENOS_VER"
    }
  }
}

data "enos_environment" "localhost" {}

resource "random_string" "cluster_id" {
  length  = 8
  lower   = true
  upper   = false
  numeric = false
  special = false
}
