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

provider "helm" "kind_enos" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_kind_enos"))
  }
}

provider "helm" "kind_enosdev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_kind_enosdev"))
  }
}

provider "helm" "oss_dev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_oss_dev"))
  }
}

provider "helm" "oss_enos" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_oss_enos"))
  }
}

provider "helm" "oss_enosdev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_oss_enosdev"))
  }
}

provider "helm" "ent_dev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ent_dev"))
  }
}

provider "helm" "ent_enos" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ent_enos"))
  }
}

provider "helm" "ent_enosdev" {
  kubernetes {
    config_path = abspath(joinpath(path.root, "kubeconfig_ent_enosdev"))
  }
}

provider "random" "default" {}
