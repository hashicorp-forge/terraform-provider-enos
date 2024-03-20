# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

output "remote_exec_all_stdout" {
  value = enos_remote_exec.all.stdout
}

output "remote_exec_all_stderr" {
  value = enos_remote_exec.all.stderr
}

output "ssh_ubuntu" {
  value = "ssh ubuntu@${aws_instance.ubuntu.public_ip}"
}

output "ssh_rhel" {
  value = "ssh ec2-user@${aws_instance.rhel.public_ip}"
}
