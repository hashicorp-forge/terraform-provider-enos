# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "public_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.this.public_ip
}

output "private_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.this.private_ip
}
