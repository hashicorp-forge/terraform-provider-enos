terraform {
  required_version = ">= 0.15.3"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
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

resource "aws_instance" "remotehost" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = "t3.micro"
  key_name      = "enos-ci-ssh-key"

  vpc_security_group_ids = [aws_security_group.this.id]

  tags = {
    Name = "enos_provider_remote_host"
  }
}

data "aws_vpc" "this" {
  default = true
}

data "enos_environment" "this" {}

resource "random_string" "security_group_suffix" {
  length  = 8
  special = false
}

resource "aws_security_group" "this" {
  name        = "ssh-access-${random_string.security_group_suffix.result}"
  description = "SSH Access"
  vpc_id      = data.aws_vpc.this.id

  # SSH
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["${data.enos_environment.this.public_ip_address}/32"]
  }

  # Allow access to external hosts
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
