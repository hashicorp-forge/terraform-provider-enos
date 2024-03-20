# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

sample "dev" {
  subset "kind" {
    matrix {
      use = ["dev"]
    }
  }

  subset "failure_handlers" {
    matrix {
      use = ["dev"]
    }
  }

  subset "vault" {
    matrix {
      use = ["dev"]
    }
  }

  subset "vault_k8s" {
    matrix {
      use = ["dev"]
    }
  }
}

sample "prod" {
  subset "kind" {
    matrix {
      use = ["prod"]
    }
  }

  subset "failure_handlers" {
    matrix {
      use = ["prod"]
    }
  }

  subset "vault" {
    matrix {
      use = ["prod"]
    }
  }

  subset "vault_k8s" {
    matrix {
      use = ["prod"]
    }
  }
}
