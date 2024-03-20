# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "consul_release" {
  type = object({
    version = string
    edition = string
  })
  description = "Consul release version and edition to install from releases.hashicorp.com"
  default = {
    version = "1.18.0"
    edition = "ce"
  }
}

variable "environment" {
  description = "The environment that the scenario is being run in"
  type        = string
  default     = "ci"
}

variable "image_repository" {
  description = "The repository for the docker image to load, i.e. hashicorp/vault"
  type        = string
  default     = null
}

variable "image_tag" {
  description = "The docker hub image tag"
  type        = string
  default     = "latest"
}

variable "instance_count" {
  description = "How many instances to create for the Vault cluster"
  type        = number
  default     = 3
}

variable "log_level" {
  description = "The server log level for Vault logs. Supported values (in order of detail) are trace, debug, info, warn, and err."
  type        = string
  default     = "info"
}

variable "run_failure_handler_tests" {
  description = "Whether or not to run the failure handlers tests"
  type        = bool
  default     = false
}

variable "tags" {
  description = "Tags to add to cloud resources"
  type        = map(string)
  default = {
    "Project Name" : "terraform-provider-enos",
    "Project" : "Enos",
    "Environment" : "ci"
  }
}
