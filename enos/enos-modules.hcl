# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

module "aws_infra" {
  source = "./modules/infra"

  project_name = "enos-provider"
  environment  = "ci"
  common_tags  = var.tags
}

module "aws_ssh_consul_cluster" {
  // source = "registry.terraform.io/hashicorp-forge/aws-consul/enos"
  // source = "../../terraform-enos-aws-consul"
  source = "./modules/aws_ssh_consul_cluster"

  project_name    = "enos-provider"
  environment     = "ci"
  common_tags     = var.tags
  ssh_aws_keypair = "enos-ci-ssh-key"

  consul_license = null
  consul_release = var.consul_release
}

module "aws_ssh_vault_cluster" {
  // source = "registry.terraform.io/hashicorp-forge/aws-vault/enos"
  // source = "../../terraform-enos-aws-vault"
  source = "./modules/aws_ssh_vault_cluster"

  project_name    = "enos-provider"
  environment     = "ci"
  common_tags     = var.tags
  ssh_aws_keypair = "enos-ci-ssh-key"
}

module "az_finder" {
  source        = "./modules/az_finder"
  instance_type = ["t3.small", "t4g.small"]
}

module "create_kind_cluster" {
  source = "./modules/create_kind_cluster"
}

module "create_vpc" {
  source = "./modules/create_vpc"
}

module "ec2_info" {
  source = "./modules/ec2_info"
}

module "environment" {
  source = "./modules/environment"
}

module "failure_handlers_setup_remote_host" {
  source = "./modules/failure_handlers_setup_remote_host"
}

module "failure_handlers_install_and_start_vault" {
  source = "./modules/failure_handlers_install_and_start_vault"
}

module "failure_handlers_install_and_start_consul" {
  source = "./modules/failure_handlers_install_and_start_consul"
}

module "helm_chart" {
  source = "./modules/helm_chart"
}

module "k8s_deploy_vault" {
  source = "./modules/k8s_deploy_vault"

  vault_instance_count = var.instance_count
}

module "k8s_verify_write_data" {
  source = "./modules/k8s_vault_verify_write_data"

  vault_instance_count = var.instance_count
}

module "kind_create_test_cluster" {
  source = "./modules/kind_create_test_cluster"
}

module "read_file" {
  source = "./modules/read_file"
}

module "target_ec2_instances" {
  source = "./modules/target_ec2_instances"

  ssh_keypair = "enos-ci-ssh-key"
}

module "test_enos_user" {
  source = "./modules/test_enos_user"
}

module "test_failure_handlers" {
  source = "./modules/test_failure_handlers"
}

module "test_host_info" {
  source = "./modules/test_host_info"
}

module "test_kind_container" {
  source = "./modules/test_kind_container"
}
