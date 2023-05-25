provider "aws" "east" {
  region = "us-east-1"
}

provider "enos" "ubuntu" {
  transport = {
    ssh = {
      user             = "ubuntu"
      private_key_path = abspath(joinpath(path.root, "./support/enos-ci-ssh-key.pem"))
    }
  }
}

provider "enos" "default" {}

provider "helm" "default" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig"))
  }
}

provider "random" "default" {}
