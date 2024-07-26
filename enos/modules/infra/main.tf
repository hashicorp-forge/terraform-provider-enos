# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.1.2"
}

resource "random_pet" "default" {
  separator = "_"
}
