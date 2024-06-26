# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }

    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "enos" {
  transport = {
    ssh = {
      user = "ubuntu"

      # You can set any of the transport settings from the environment,
      # e.g.: ENOS_TRANSPORT_PRIVATE_KEY_PATH=/path/to/key.pem

      # private_key_path = "/Users/ryan/.ssh/id_rsa"
    }
  }
}

provider "aws" {
  region = "us-west-2"
}

locals {
  foo_template_rendered = templatefile("${path.module}/files/foo.tmpl", {
    "foo" = var.input_test
  })
}

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "all" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

data "aws_ami" "rhel" {
  most_recent = true

  # Currently latest latest point release-1
  filter {
    name   = "name"
    values = ["RHEL-8.2*HVM-20*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["309956199498"] # Redhat
}

data "enos_environment" "localhost" {
}

module "target_sg" {
  source = "terraform-aws-modules/security-group/aws//modules/ssh"

  name        = "enos_core_example"
  description = "Enos provider core example security group"
  vpc_id      = data.aws_vpc.default.id

  ingress_cidr_blocks = ["${data.enos_environment.localhost.public_ip_address}/32"]
}

resource "aws_instance" "ubuntu" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = "t3.micro"
  key_name                    = var.key_name
  associate_public_ip_address = true
  security_groups             = [module.target_sg.security_group_name]
}

resource "aws_instance" "rhel" {
  ami                         = data.aws_ami.rhel.id
  instance_type               = "t3.micro"
  key_name                    = var.key_name
  associate_public_ip_address = true
  security_groups             = [module.target_sg.security_group_name]
}

resource "enos_file" "from_source" {
  depends_on = [aws_instance.ubuntu]

  source      = "${path.module}/files/foo.txt"
  destination = "/tmp/from_source.txt"

  transport = {
    ssh = {
      host = aws_instance.ubuntu.public_ip
    }
  }
}

resource "enos_file" "from_content" {
  depends_on = [aws_instance.ubuntu]

  content     = local.foo_template_rendered
  destination = "/tmp/from_content.txt"

  transport = {
    ssh = {
      host = aws_instance.ubuntu.public_ip
    }
  }
}

resource "enos_remote_exec" "all" {
  depends_on = [
    aws_instance.ubuntu,
    enos_file.from_source,
    enos_file.from_content,
  ]

  environment = {
    HELLO_WORLD = "Hello, World!"
  }

  inline  = ["rm /tmp/from_source.txt /tmp/from_content.txt"]
  scripts = ["${path.module}/files/script.sh"]
  content = local.foo_template_rendered

  transport = {
    ssh = {
      host = aws_instance.ubuntu.public_ip
    }
  }
}

resource "enos_bundle_install" "deb" {
  depends_on = [aws_instance.ubuntu]

  path = abspath("${path.module}/packages/enostest.deb")

  transport = {
    ssh = {
      host = aws_instance.ubuntu.public_ip
    }
  }
}

resource "enos_bundle_install" "rpm" {
  depends_on = [aws_instance.rhel]

  path = abspath("${path.module}/packages/enostest-1.0.0-1.src.rpm")

  transport = {
    ssh = {
      user = "ec2-user"
      host = aws_instance.rhel.public_ip
    }
  }
}
