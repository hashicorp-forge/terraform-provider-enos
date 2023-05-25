terraform_cli "default" {
  provider_installation {
    dev_overrides = {
      "app.terraform.io/hashicorp-qti/enos" = abspath(joinpath(path.root, "../dist"))
    }
    direct {}
  }

  credentials "app.terraform.io" {
    token = var.tfc_api_token
  }
}

terraform "default" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }

    aws = {
      source = "hashicorp/aws"
    }

    random = {
      source = "hashicorp/random"
    }
  }
}

terraform "k8s" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enos"
    }

    helm = {
      source = "hashicorp/helm"
    }
  }
}
