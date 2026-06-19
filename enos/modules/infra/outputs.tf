# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "aws_region" {
  description = "AWS Region for resources"
  value       = data.aws_region.current.region
}

output "vpc_id" {
  description = "Created VPC ID"
  value       = aws_vpc.enos_vpc.id
}

output "vpc_cidr" {
  description = "CIDR for whole VPC"
  value       = var.vpc_cidr
}

output "vpc_subnets" {
  description = "Generated subnet IDs and CIDRs"
  value       = { for s in aws_subnet.enos_subnet : s.id => s.cidr_block }
}

output "ami_ids" {
  description = "The AWS AMI IDs for to use for ubuntu and rhel based instance for the amd64 and arm64 architectures."
  value = {
    ubuntu = { for arch in local.architecture_filters : arch => data.aws_ami.ubuntu[arch].id }
    rhel   = { for arch in local.architecture_filters : arch => data.aws_ami.rhel[arch].id }
  }
}

output "availability_zone_names" {
  description = "All availability zones with resources"
  value       = data.aws_availability_zones.available.names
}

output "account_id" {
  description = "AWS account ID"
  value       = data.aws_caller_identity.current.account_id
}

output "kms_key_arn" {
  description = "ARN of the generated KMS key"
  value       = aws_kms_key.enos_key.arn
}

output "kms_key_alias" {
  description = "Alias of the generated KMS key"
  value       = aws_kms_alias.enos_key_alias.name
}

output "pet_id" {
  description = "The ID of the random_pet used in this module"
  value       = random_pet.default.id
}
