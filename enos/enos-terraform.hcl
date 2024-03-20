# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform_cli "default" {
}

terraform_cli "dev" {
  /*
  // TODO(post-publish) We can't use dev_overrides until some version of this is in the registry
  // Until that time we'll need to install the binaries into the implied local mirror
  // https://developer.hashicorp.com/terraform/cli/config/config-file#implied-local-mirror-directories

  provider_installation {
    dev_overrides = {
      "hashicorp-forge/enos" = abspath(joinpath(path.root, "../dist"))
    }
    direct {}
  */
}

terraform "default" {
  required_version = ">= 1.2.0"

  required_providers {
    enos = {
      source = "registry.terraform.io/hashicorp-forge/enos"
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
      source = "registry.terraform.io/hashicorp-forge/enos"
    }

    helm = {
      source = "hashicorp/helm"
    }
  }
}
