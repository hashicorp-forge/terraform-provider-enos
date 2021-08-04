terraform {
  required_version = ">= 0.15.3"

  backend "remote" {}

  required_providers {
    # We need to specify the provider source in each module until we publish it
    # to the public registry
    # We substitute the ENOS_VER in CI with the Artifact version being tested
    enos = {
      version = "= ENOS_VER"
      source  = "hashicorp.com/qti/enos"
    }
  }
}

provider "aws" {
  alias  = "east"
  region = "us-east-1"
}

provider "enos" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = "./enos-ci-ssh-keypair.pem"
    }
  }
}

module "enos_infra" {
  source = "app.terraform.io/hashicorp-qti/aws-infra/enos"
  providers = {
    aws = aws.east
  }

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags = {
    "Project Name" : "qti-enos-provider",
    "Environment" : "ci"
  }
}

module "consul_cluster" {
  source = "app.terraform.io/hashicorp-qti/aws-consul/enos"

  depends_on = [module.enos_infra]

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags = {
    "Project Name" : "qti-enos-provider",
    "Environment" : "ci"
  }
  ssh_aws_keypair = "enos-ci-ssh-keypair"
  ubuntu_ami_id   = module.enos_infra.ubuntu_ami_id
  vpc_id          = module.enos_infra.vpc_id
  kms_key_arn     = module.enos_infra.kms_key_arn
  consul_license  = "none"
}

module "vault_cluster" {
  source = "app.terraform.io/hashicorp-qti/aws-vault/enos"

  depends_on = [module.enos_infra]

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags = {
    "Project Name" : "qti-enos-provider",
    "Environment" : "ci"
  }
  ssh_aws_keypair = "enos-ci-ssh-keypair"
  ubuntu_ami_id   = module.enos_infra.ubuntu_ami_id
  vpc_id          = module.enos_infra.vpc_id
  kms_key_arn     = module.enos_infra.kms_key_arn
  consul_ips      = module.consul_cluster.instance_private_ips
  vault_license   =file("/tmp/vault.hclic")
  vault_release = {
    version = "1.8.0"
    edition = "ent"
  }
}
