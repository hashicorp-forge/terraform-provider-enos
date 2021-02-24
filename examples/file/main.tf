terraform {
  required_providers {
    enos = {
      version = "~> 0.1"
      source  = "hashicorp.com/qti/enos"
    }

    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "enos" { }

provider "aws" {
  region = "us-west-2"
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

resource "aws_instance" "target" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = "t3.micro"
  key_name                    = var.key_name
  associate_public_ip_address = true
  security_groups             = [var.security_group]
}

resource "enos_file" "foo" {
  depends_on = [aws_instance.target]

  source      = "/tmp/foo"
  destination = "/tmp/bar"
  transport   = {
    ssh                = {
      user             = "ubuntu"
      host             = aws_instance.target.public_ip
      private_key_path = var.key_path
    }
  }
}

/*
data "enos_transport" "target" {
  depends_on = [aws_instance.target]

  ssh {
    user = "ubuntu"
    host = aws_instance.target.public_ip
    private_key_path = var.key_path
  }
}

resource "enos_file" "foo" {
  depends_on = [data.enos_transport.target]

  source      = "/tmp/foo"
  destination = "/tmp/enos/bar"
  transport   = data.enos_transport.target.out
}
*/
