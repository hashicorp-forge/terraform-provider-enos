terraform {
  required_version = ">= 1.1.2"

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

locals {
  name_suffix        = "${var.project_name}-${var.environment}"
  consul_bin_path    = "${var.consul_install_dir}/consul"
  consul_cluster_tag = "consul-server-${random_string.cluster_id.result}"
}

resource "random_string" "cluster_id" {
  length  = 8
  lower   = true
  upper   = false
  numeric = false
  special = false
}
