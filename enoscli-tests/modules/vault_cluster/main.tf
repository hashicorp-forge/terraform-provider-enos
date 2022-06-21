terraform {
  required_providers {
    enosdev = {
      source = "app.terraform.io/hashicorp-qti/enosdev"
      version = "ENOS_VER"
    }

    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
      version = "0.1.29"
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
