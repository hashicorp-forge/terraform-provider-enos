terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }
    aws = {
      source = "hashicorp/aws"
    }
  }
}

data "aws_ami" "ubuntu" {
  most_recent = true

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
    values = ["x86_64"]
  }

  owners = ["099720109477"] # Canonical
}

resource "aws_instance" "this" {
  ami           = data.aws_ami.ubuntu.id
  instance_type = var.instance_type
  key_name      = "enos-ci-ssh-key"

  subnet_id = var.subnet_id

  vpc_security_group_ids = [aws_security_group.this.id]

  tags = var.tags
}

data "enos_environment" "this" {}

resource "random_string" "security_group_suffix" {
  length  = 8
  special = false
}

resource "aws_security_group" "this" {
  name        = "ssh-access-${random_string.security_group_suffix.result}"
  description = "SSH Access"
  vpc_id      = var.vpc_id

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
