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

data "enos_environment" "localhost" {}

output "public_ip_address" {
  value = data.enos_environment.localhost.public_ip_address
}

output "public_ip_addresses" {
  value = data.enos_environment.localhost.public_ip_addresses
}

output "public_ipv4_addresses" {
  value = data.enos_environment.localhost.public_ipv4_addresses
}

output "public_ipv6_addresses" {
  value = data.enos_environment.localhost.public_ipv6_addresses
}
