output "remote_exec_all_stdout" {
  value = enos_remote_exec.all.stdout
}

output "remote_exec_all_stderr" {
  value = enos_remote_exec.all.stderr
}

output "ssh" {
  value = "ssh ubuntu@${aws_instance.target.public_ip}"
}
