# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

data "aws_availability_zones" "available" {
  state = "available"
  filter {
    name   = "zone-name"
    values = var.availability_zones
  }
}

locals {
  // AWS AMIs standardized on the x86_64 label for 64bit x86 architectures, therefore amd64 should be rather x86_64.
  architecture_filters = [for arch in var.ami_architectures : (arch == "amd64" ? "x86_64" : arch)]
  common_tags = merge(
    var.common_tags,
    {
      "Module" = "terraform-enos-aws-infra"
      "Pet"    = random_pet.default.id
    },
  )
}

data "aws_ami" "ubuntu" {
  most_recent = true
  count       = length(local.architecture_filters)

  # Currently latest LTS-1
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-*-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [local.architecture_filters[count.index]]
  }

  owners = ["099720109477"] # Canonical
}

data "aws_ami" "rhel" {
  most_recent = true
  count       = length(local.architecture_filters)

  filter {
    name   = "name"
    values = ["RHEL-8.8*HVM-20*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [local.architecture_filters[count.index]]
  }

  owners = ["309956199498"] # Redhat
}

resource "random_string" "cluster_id" {
  length  = 8
  lower   = true
  upper   = false
  numeric = false
  special = false
}
