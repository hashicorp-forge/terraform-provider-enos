# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "ami_id" {
  description = "AMI from enos-infra"
  type        = string
}

variable "common_tags" {
  description = "Tags to set for all resources"
  type        = map(string)
  default     = { "Project" : "enos-provider" }
}

variable "consul_config_dir" {
  type        = string
  description = "The directory where the consul will write config files"
  default     = "/etc/consul.d"
}

variable "consul_data_dir" {
  type        = string
  description = "The directory where the consul will store data"
  default     = "/opt/consul/data"
}

variable "consul_install_dir" {
  type        = string
  description = "The directory where the consul binary will be installed"
  default     = "/opt/consul/bin"
}

variable "consul_license" {
  type        = string
  sensitive   = true
  description = "The consul enterprise license"
  default     = null
}

variable "consul_log_dir" {
  type        = string
  description = "The directory where the consul will write log files"
  default     = "/var/log/consul.d"
}

variable "consul_log_level" {
  type        = string
  description = "The consul service log level"
  default     = "info"

  validation {
    condition     = contains(["trace", "debug", "info", "warn", "error"], var.consul_log_level)
    error_message = "The vault_log_level must be one of 'trace', 'debug', 'info', 'warn', or 'error'."
  }
}

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

variable "environment" {
  description = "Name of the environment."
  type        = string
}

variable "instance_count" {
  description = "Number of EC2 instances in each subnet"
  type        = number
  default     = 3
}

variable "instance_type" {
  description = "EC2 Instance"
  type        = string
  default     = "t2.micro"
}

variable "kms_key_arn" {
  type        = string
  description = "ARN of KMS Key from enos-infra"
}

variable "project_name" {
  description = "Name of the project."
  type        = string
}

variable "ssh_aws_keypair" {
  description = "SSH keypair used to connect to EC2 instances"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID from enos-infra"
  type        = string
}
