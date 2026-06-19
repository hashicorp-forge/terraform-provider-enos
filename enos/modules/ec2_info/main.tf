# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

locals {
  architectures      = toset(["arm64", "x86_64"])
  canonical_owner_id = "099720109477"
  rhel_owner_id      = "309956199498"
  ids = {
    "arm64" = {
      "rhel" = {
        "9.8"  = data.aws_ami.rhel_98["arm64"].id
        "10.2" = data.aws_ami.rhel_102["arm64"].id
      }
      "ubuntu" = {
        "22.04" = data.aws_ami.ubuntu_2204["arm64"].id
        "24.04" = data.aws_ami.ubuntu_2404["arm64"].id
        "26.04" = data.aws_ami.ubuntu_2604["arm64"].id
      }
    }
    "amd64" = {
      "rhel" = {
        "9.8"  = data.aws_ami.rhel_98["x86_64"].id
        "10.2" = data.aws_ami.rhel_102["x86_64"].id
      }
      "ubuntu" = {
        "22.04" = data.aws_ami.ubuntu_2204["x86_64"].id
        "24.04" = data.aws_ami.ubuntu_2404["x86_64"].id
        "26.04" = data.aws_ami.ubuntu_2604["x86_64"].id
      }
    }
  }
}

data "aws_ami" "ubuntu_2204" {
  most_recent = true
  for_each    = local.architectures

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-*-22.04-*-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [each.value]
  }

  owners = [local.canonical_owner_id]
}

data "aws_ami" "ubuntu_2404" {
  most_recent = true
  for_each    = local.architectures

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-*-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [each.value]
  }

  owners = [local.canonical_owner_id]
}

data "aws_ami" "ubuntu_2604" {
  most_recent = true
  for_each    = local.architectures

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-resolute-26.04-*-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [each.value]
  }

  owners = [local.canonical_owner_id]
}

data "aws_ami" "rhel_98" {
  most_recent = true
  for_each    = local.architectures

  # Currently latest latest point release-1
  filter {
    name   = "name"
    values = ["RHEL-9.8*HVM-20*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [each.value]
  }

  owners = [local.rhel_owner_id]
}

data "aws_ami" "rhel_102" {
  most_recent = true
  for_each    = local.architectures

  # Currently latest latest point release-1
  filter {
    name   = "name"
    values = ["RHEL-10.2*HVM-20*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [each.value]
  }

  owners = [local.rhel_owner_id]
}


data "aws_region" "current" {}

data "aws_availability_zones" "available" {
  state = "available"

  filter {
    name   = "zone-name"
    values = ["*"]
  }
}

output "ami_ids" {
  value = local.ids
}

output "current_region" {
  value = data.aws_region.current
}

output "availability_zones" {
  value = data.aws_availability_zones.available
}
