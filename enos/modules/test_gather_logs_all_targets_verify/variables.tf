# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variable "debug_data_root_dir" {
  description = "Local directory where the enos provider writes failure-handler log files. Must match the debug_data_root_dir configured on provider.enos.ubuntu_with_debug."
  type        = string
}
