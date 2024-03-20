# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "host_public_ip" {
  description = "The public ip address of the host to run a failing remote_exec task on"
  type        = string
}
