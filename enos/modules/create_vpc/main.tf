terraform {
  required_version = ">= 1.2.0"

  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

locals {
  vpc_cidr          = "10.13.0.0/16"
  subnet_cidr       = "10.13.10.0/24"
  availability_zone = sort(data.aws_ec2_instance_type_offerings.infra.locations)[0]
}

data "aws_ec2_instance_type_offerings" "infra" {
  filter {
    name   = "instance-type"
    values = [var.instance_type]
  }

  location_type = "availability-zone"
}

resource "aws_vpc" "enos_vpc" {
  cidr_block = local.vpc_cidr
  # Enabled for RDS
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = var.tags
}

resource "aws_subnet" "enos_subnet" {
  vpc_id                  = aws_vpc.enos_vpc.id
  cidr_block              = local.subnet_cidr
  map_public_ip_on_launch = true
  availability_zone       = local.availability_zone

  tags = var.tags
}

resource "aws_internet_gateway" "enos_gw" {
  vpc_id = aws_vpc.enos_vpc.id
  tags   = var.tags
}

resource "aws_route_table" "enos_route" {
  vpc_id = aws_vpc.enos_vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.enos_gw.id
  }

  tags = var.tags
}

resource "aws_route_table_association" "enos_crta" {
  subnet_id      = aws_subnet.enos_subnet.id
  route_table_id = aws_route_table.enos_route.id
}
