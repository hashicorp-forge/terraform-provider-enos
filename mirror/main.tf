variable "aws_region" {
  type = string
}

variable "bucket" {
  type = string
}

variable "allowed_ips" {
  type = list(string)
}

terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }

  backend "remote" {
    hostname     = "app.terraform.io"
    organization = "hashicorp-qti"

    workspaces {
      name = "enos-provider-bucket"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_iam_policy_document" "enos_provider" {
  statement {
    effect = "Allow"

    actions = [
      "s3:*"
    ]

    resources = [
      "arn:aws:s3:::${var.bucket}",
      "arn:aws:s3:::${var.bucket}/*"
    ]

    condition {
      test = "IpAddress"
      variable = "aws:SourceIP"
      values = var.allowed_ips
    }

    principals {
      type = "*"
      identifiers = ["*"]
    }
  }

  statement {
    effect = "Allow"

    actions = [
      "s3:GetObject"
    ]

    resources = [
      "arn:aws:s3:::${var.bucket}",
      "arn:aws:s3:::${var.bucket}/*"
    ]

     principals {
      type = "*"
      identifiers = ["*"]
    }
  }
}

resource "aws_s3_bucket" "enos-provider" {
  bucket = "enos-provider"
  acl    = "private"
  policy = data.aws_iam_policy_document.enos_provider.json

  website {
    index_document = "index.json"
  }

  tags = {
    hc-internet-facing = true
  }
}
