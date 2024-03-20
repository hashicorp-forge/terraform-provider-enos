# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

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

data "enos_environment" "localhost" {
}
