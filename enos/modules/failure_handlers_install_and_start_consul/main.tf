terraform {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }
  }
}

locals {
  release = {
    product = "consul"
    version = "1.10.3"
    edition = "ce"
  }

  install_dir = "/opt/consul/bin"
  bin_path    = "${local.install_dir}/consul"
  data_dir    = "/opt/consul/data"
  config_dir  = "/etc/consul.d"
  log_dir     = "/var/log/consul.d"
}

resource "enos_bundle_install" "consul" {
  destination = local.install_dir
  release     = local.release

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}

resource "enos_consul_start" "consul" {
  depends_on = [
    enos_bundle_install.consul,
  ]

  bin_path   = local.bin_path
  data_dir   = local.data_dir
  config_dir = local.config_dir
  config = {
    datacenter       = "dc1"
    retry_join       = ["127.0.0.1"]
    data_dir         = local.data_dir
    log_level        = "INFO"
    server           = true
    bootstrap_expect = 1
    log_file         = "log_dir"
  }
  unit_name = "consul"
  username  = "consul"

  transport = {
    ssh = {
      host = var.host_public_ip
    }
  }
}
