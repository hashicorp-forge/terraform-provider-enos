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

resource "enos_remote_exec" "enosallgroup" {
  inline = ["sudo groupadd -g 900 -fo enosall"]

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

resource "enos_user" "all" {
  depends_on = [enos_remote_exec.enosallgroup]

  name     = "enosall"
  home_dir = "/home/enosall"
  shell    = "/bin/false"
  uid      = "900"
  gid      = "900"

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

resource "enos_user" "min" {
  name = "enosmin"

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

resource "enos_remote_exec" "verify_users" {
  depends_on = [
    enos_user.all,
    enos_user.min,
  ]

  scripts = [abspath("${path.module}/scripts/verify-users.sh")]

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}
