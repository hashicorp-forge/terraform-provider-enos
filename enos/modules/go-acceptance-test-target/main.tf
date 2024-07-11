# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 0.15.3"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
    }
  }
}

provider "enos" {}

provider "aws" {
  region = "us-east-1"
}

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"] # Canonical
}

data "aws_vpc" "this" {
  default = true
}

data "enos_environment" "this" {}

variable "ssh_key_name" {
  type        = string
  description = "The name of the private ssh keypair to use"
}

variable "private_key_path" {
  type        = string
  description = "The path to the private ssh key"
}

resource "random_string" "security_group_suffix" {
  length  = 8
  special = false
}

resource "aws_security_group" "this" {
  name        = "ssh-access-${random_string.security_group_suffix.result}"
  description = "SSH Access"
  vpc_id      = data.aws_vpc.this.id

  # SSH
  dynamic "ingress" {
    for_each = data.enos_environment.this.public_ipv4_addresses

    content {
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = ["${ingress.value}/32"]
    }
  }

  # Allow access to external hosts
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "remotehost" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = "t3.small"
  key_name      = var.ssh_key_name

  vpc_security_group_ids = [aws_security_group.this.id]

  tags = {
    Name = "enos_provider_remote_host"
  }
}

resource "enos_remote_exec" "wait" {
  inline = ["sudo cloud-init status --wait"]

  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = abspath(var.private_key_path)
      host             = aws_instance.remotehost.public_ip
    }
  }
}
