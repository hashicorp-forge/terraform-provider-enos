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

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "all" {
  vpc_id = data.aws_vpc.default.id
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

data "enos_environment" "localhost" {
}

data "template_file" "foo_template" {
  template = file("${path.module}/files/foo.tmpl")

  vars = {
    "foo" = var.input_test
  }
}

module "target_sg" {
  source = "terraform-aws-modules/security-group/aws//modules/ssh"

  name        = "enos_core_example"
  description = "Enos provider core example security group"
  vpc_id      = data.aws_vpc.default.id

  ingress_cidr_blocks = ["${data.enos_environment.localhost.public_ip_address}/32"]
}

resource "aws_instance" "target" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = "t3.micro"
  key_name                    = var.key_name
  associate_public_ip_address = true
  security_groups             = [module.target_sg.security_group_name]
}

resource "enos_file" "from_source" {
  depends_on = [aws_instance.target]

  source      = "${path.module}/files/foo.txt"
  destination = "/tmp/from_source.txt"

  transport = {
    ssh = {
      host = aws_instance.target.public_ip
    }
  }
}

resource "enos_file" "from_content" {
  depends_on = [aws_instance.target]

  content     = data.template_file.foo_template.rendered
  destination = "/tmp/from_content.txt"

  transport = {
    ssh = {
      host = aws_instance.target.public_ip
    }
  }
}

resource "enos_remote_exec" "all" {
  depends_on = [
    aws_instance.target,
    enos_file.from_source,
    enos_file.from_content,
  ]

  environment = {
    HELLO_WORLD = "Hello, World!"
  }

  inline  = ["rm /tmp/from_source.txt /tmp/from_content.txt"]
  scripts = ["${path.module}/files/script.sh"]
  content = data.template_file.foo_template.rendered

  transport = {
    ssh = {
      host = aws_instance.target.public_ip
    }
  }
}
