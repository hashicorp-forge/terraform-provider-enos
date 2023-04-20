module "setup_remote_host" {
  source = abspath("./modules/setup_remote_host")
}

module "install_and_start_vault" {
  source = abspath("./modules/install_and_start_vault")
}

module "install_and_start_consul" {
  source = abspath("./modules/install_and_start_consul")
}

module "test_failure_handlers" {
  source = abspath("./modules/test_failure_handlers")
}

module "create_vpc" {
  source = abspath("./modules/create_vpc")
}

module "environment" {
  source = abspath("./modules/environment")
}
