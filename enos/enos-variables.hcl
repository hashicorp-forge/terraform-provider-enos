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
