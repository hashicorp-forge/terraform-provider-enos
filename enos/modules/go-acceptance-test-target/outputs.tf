# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "instance_public_ip" {
  description = "Public IP of remote host"
  value       = aws_instance.remotehost.public_ip
}
