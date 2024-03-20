# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "vpc_id" {
  description = "The id of a vpc to create the remote host in. If not provided the default vpc will be used"
  type        = string
}

variable "tags" {
  description = "The tags to tag the remote host with"
  type        = map(string)
  default     = {}
}

variable "instance_type" {
  description = "The instance type to provision"
  type        = string
  default     = "t3.micro"
}
