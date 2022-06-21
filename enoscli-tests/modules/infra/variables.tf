variable "vpc_name" {
  type        = string
  default     = "enos-vpc"
  description = "Descriptive name of the VPC"
}

variable "availability_zones" {
  description = "List of AWS availability zones to use (or * for all available)"
  type        = list(string)
  default     = ["*"]
}

variable "vpc_cidr" {
  type        = string
  default     = "10.13.0.0/16"
  description = "CIDR for the VPC"
}

variable "project_name" {
  description = "Name of the project."
  type        = string
}

variable "environment" {
  description = "Name of the environment."
  type        = string
}

variable "common_tags" {
  description = "Tags to set for all resources"
  type        = map(string)
  default     = { "Project" : "Enos" }
}

variable "ami_architectures" {
  type        = list(string)
  description = "The AMI architectures to fetch AMI IDs for."
  default     = ["amd64", "arm64"]
}
