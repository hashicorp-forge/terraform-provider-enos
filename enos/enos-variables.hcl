variable "environment" {
  description = "The environment that the scenario is being run in"
  type        = string
  default     = "ci"
}

variable "run_failure_handler_tests" {
  description = "Whether or not to run the failure handlers tests"
  type        = bool
  default     = false
}

variable "tfc_api_token" {
  description = "The Terraform Cloud QTI Organization API token."
  type        = string
  sensitive   = true
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

variable "log_level" {
  description = "The server log level for Vault logs. Supported values (in order of detail) are trace, debug, info, warn, and err."
  type        = string
  default     = "info"
}

variable "instance_count" {
  description = "How many instances to create for the Vault cluster"
  type        = number
  default     = 3
}
