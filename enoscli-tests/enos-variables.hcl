variable "tfc_api_token" {
  description = "The Terraform Cloud QTI Organization API token."
  type        = string
}

variable "enos_provider_name" {
  description = "The Enos Provider private registry name"
  type        = string
  default     = "enosdev"
}

variable "enos_provider_version" {
  description = "The Enos Provider version"
  type        = string
}
