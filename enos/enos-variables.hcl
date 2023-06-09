variable "consul_release" {
  type = object({
    version = string
    edition = string
  })
  description = "Consul release version and edition to install from releases.hashicorp.com"
  default = {
    version = "1.15.3"
    edition = "oss"
  }
}

variable "enosdev_provider_version" {
  description = "The version of the enosdev provider to install for enosdev scenarios"
  type        = string
  default     = "0.3.26"
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
    "Project Name" : "enos-provider",
    "Project" : "Enos",
    "Environment" : "ci"
  }
}

variable "tfc_api_token" {
  description = "The Terraform Cloud QTI Organization API token."
  type        = string
  sensitive   = true
}
