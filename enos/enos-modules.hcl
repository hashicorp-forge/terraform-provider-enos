module "setup_remote_host" {
  source = "./modules/setup_remote_host"
}

module "install_and_start_vault" {
  source = "./modules/install_and_start_vault"
}

module "install_and_start_consul" {
  source = "./modules/install_and_start_consul"
}

module "test_failure_handlers" {
  source = "./modules/test_failure_handlers"
}

module "create_vpc" {
  source = "./modules/create_vpc"
}

module "environment" {
  source = "./modules/environment"
}

module "create_kind_cluster" {
  source = "./modules/local_kind_cluster"
}

module "load_docker_image" {
  source = "./modules/load_docker_image"
}

module "k8s_deploy_vault" {
  source = "./modules/k8s_deploy_vault"

  vault_instance_count = var.instance_count
}

module "k8s_verify_write_data" {
  source = "./modules/k8s_vault_verify_write_data"

  vault_instance_count = var.instance_count
}

module "read_license" {
  source = "./modules/read_license"
}
