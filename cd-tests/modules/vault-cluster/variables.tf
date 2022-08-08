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

variable "enos_transport_user" {
  description = "Enos transport username. If unset the provider level configuration will be used"
  type        = string
  default     = null
}

variable "ami_id" {
  description = "AMI from enos-infra"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID from enos-infra"
  type        = string
}

variable "consul_cluster_tag" {
  type        = string
  description = "cluster tag for consul cluster"
  default     = null
}

variable "kms_key_arn" {
  type        = string
  description = "ARN of KMS Key from enos-infra"
}

variable "manage_service" {
  type        = bool
  description = "Manage the service users and systemd"
  default     = true
}

variable "vault_license" {
  type        = string
  sensitive   = true
  description = "vault license"
  default     = null
}

variable "vault_release" {
  type = object({
    version = string
    edition = string
  })
  description = "Vault release version and edition to install from releases.hashicorp.com"
  default     = null
}

variable "vault_artifactory_release" {
  type = object({
    username = string
    token    = string
    url      = string
    sha256   = string
  })
  description = "Vault release version and edition to install from artifactory.hashicorp.engineering"
  default     = null
}

variable "vault_local_artifact_path" {
  type        = string
  description = "The path to a locally built vault artifact to install"
  default     = null
}

variable "vault_log_dir" {
  type        = string
  description = "The directory to use for Vault logs"
  default     = "/var/log/vault.d"
}

variable "vault_config_dir" {
  type        = string
  description = "The directory to use for Vault configuration"
  default     = "/etc/vault.d"
}

variable "vault_install_dir" {
  type        = string
  description = "The directory where the vault binary will be installed"
  default     = "/opt/vault/bin"
}

variable "consul_release" {
  type = object({
    version = string
    edition = string
  })
  description = "Consul release version and edition to install from releases.hashicorp.com"
  default = {
    version = "1.10.3"
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

variable "consul_log_dir" {
  type        = string
  description = "The directory where the consul will write log output"
  default     = "/var/log/consul.d"
}

variable "dependencies_to_install" {
  type        = list(string)
  description = "A list of dependencies to install"
  default     = []
}

variable "storage_backend" {
  type        = string
  description = "The type of Vault storage backend which will be used"
  default     = "raft"
  validation {
    condition     = contains(["raft", "consul"], var.storage_backend)
    error_message = "The \"storage_backend\" must be one of: [raft|consul]."
  }
}

variable "storage_backend_addl_config" {
  type        = map(any)
  description = "A set of key value pairs to inject into the storage block"
  default     = {}
}

variable "vault_cluster_tag" {
  type        = string
  description = "Cluster tag for Vault cluster"
  default     = null
}

variable "vault_root_token" {
  type = string
  #sensitive   = true
  description = "Vault root token"
  default     = null
}

variable "vault_node_prefix" {
  type        = string
  description = "The vault node prefix"
  default     = "node"
}

variable "vault_init" {
  type        = bool
  description = "Initialize vault"
  default     = "true"
}
