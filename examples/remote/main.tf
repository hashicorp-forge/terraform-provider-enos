terraform {
  required_providers {
    enos = {
      version = "~> 0.1.0"
      source  = "hashicorp.com/qti/enos"
    }
  }
}

provider "enos" {
}

data "enos_environment" "localhost" {
}
