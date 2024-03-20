# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform_cli "default" {
  credentials "app.terraform.io" {
    token = var.tfc_api_token
  }
}

terraform_cli "dev" {
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

terraform "enosdev" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source  = "app.terraform.io/hashicorp-qti/enosdev"
      version = var.enosdev_provider_version
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

terraform "k8s_enosdev" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "app.terraform.io/hashicorp-qti/enosdev"
    }

    helm = {
      source = "hashicorp/helm"
    }
  }
}
