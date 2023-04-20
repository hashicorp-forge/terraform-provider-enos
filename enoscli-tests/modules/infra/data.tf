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
}

data "aws_ami" "ubuntu" {
  most_recent = true
  count       = length(local.architecture_filters)

  # Currently latest LTS-1
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-*-server-*"]
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

  # Currently latest latest point release-1
  filter {
    name   = "name"
    values = ["RHEL-8.2*HVM-20*"]
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

data "aws_vpc" "infra" {
  id = aws_vpc.enos_vpc.id
}

resource "random_string" "cluster_id" {
  length  = 8
  lower   = true
  upper   = false
  numeric = false
  special = false
}
