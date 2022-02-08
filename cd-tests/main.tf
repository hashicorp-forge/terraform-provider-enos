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
  version = ">= 0.1.0"

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags = {
    "Project Name" : "qti-enos-provider",
    "Environment" : "ci"
  }
  ami_architectures = ["amd64"]
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
  ami_id          = module.enos_infra.ami_ids["ubuntu"]["amd64"]
  vpc_id          = module.enos_infra.vpc_id
  kms_key_arn     = module.enos_infra.kms_key_arn
  consul_license  = "none"
}

module "vault_cluster" {
  source = "app.terraform.io/hashicorp-qti/aws-vault/enos"

  depends_on = [
    module.enos_infra,
    module.consul_cluster,
  ]

  project_name = "qti-enos-provider"
  environment  = "ci"
  common_tags = {
    "Project Name" : "qti-enos-provider",
    "Environment" : "ci"
  }
  ssh_aws_keypair    = "enos-ci-ssh-keypair"
  ami_id             = module.enos_infra.ami_ids["ubuntu"]["amd64"]
  vpc_id             = module.enos_infra.vpc_id
  kms_key_arn        = module.enos_infra.kms_key_arn
  consul_cluster_tag = module.consul_cluster.consul_cluster_tag
  vault_license      = file("/tmp/vault.hclic")
  vault_release = {
    version = "1.8.5"
    edition = "ent"
  }
}
