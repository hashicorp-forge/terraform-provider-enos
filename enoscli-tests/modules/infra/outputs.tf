output "aws_region" {
  description = "AWS Region for resources"
  value       = data.aws_region.current.name
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
    ubuntu = { for idx, arch in var.ami_architectures : arch => data.aws_ami.ubuntu[idx].id }
    rhel   = { for idx, arch in var.ami_architectures : arch => data.aws_ami.rhel[idx].id }
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
