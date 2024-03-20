# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

provider "aws" "default" {
  region = "us-east-1"
}

provider "enos" "default" {}

provider "enos" "ubuntu" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = abspath(joinpath(path.root, "./support/enos-ci-ssh-key.pem"))
    }
  }
}

provider "enos" "rhel" {
  transport = {
    ssh = {
      user             = "ec2-user"
      private_key_path = abspath(joinpath(path.root, "./support/enos-ci-ssh-key.pem"))
    }
  }
}

provider "helm" "default" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig"))
  }
}

provider "helm" "kind_dev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_kind_dev"))
  }
}

provider "helm" "kind_prod" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_kind_prod"))
  }
}

provider "helm" "ce_dev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ce_dev"))
  }
}

provider "helm" "ce_prod" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ce_prod"))
  }
}

provider "helm" "ent_dev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ent_dev"))
  }
}

provider "helm" "ent_prod" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ent_prod"))
  }
}

provider "random" "default" {}
