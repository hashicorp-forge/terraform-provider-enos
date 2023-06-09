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
  default     = { "Project" : "enos-provider" }
}

variable "instance_type" {
  description = "EC2 Instance"
  type        = string
  default     = "t2.micro"
}

variable "instance_count" {
  description = "Number of EC2 instances in each subnet"
  type        = number
  default     = 3
}

variable "ssh_aws_keypair" {
  description = "SSH keypair used to connect to EC2 instances"
  type        = string
}

variable "ami_id" {
  description = "AMI from enos-infra"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID from enos-infra"
  type        = string
}

variable "kms_key_arn" {
  type        = string
  description = "ARN of KMS Key from enos-infra"
}

variable "consul_license" {
  type        = string
  sensitive   = true
  description = "The consul enterprise license"
  default     = null
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

variable "consul_install_dir" {
  type        = string
  description = "The directory where the consul binary will be installed"
  default     = "/opt/consul/bin"
}

variable "consul_data_dir" {
  type        = string
  description = "The directory where the consul will store data"
  default     = "/opt/consul/data"
}

variable "consul_config_dir" {
  type        = string
  description = "The directory where the consul will write config files"
  default     = "/etc/consul.d"
}

variable "consul_log_dir" {
  type        = string
  description = "The directory where the consul will write log files"
  default     = "/var/log/consul.d"
}
