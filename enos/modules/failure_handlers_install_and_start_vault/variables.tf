variable "host_public_ip" {
  description = "The public ip address of the host to install vault on"
  type        = string
}

variable "host_private_ip" {
  description = "The private ip of the host that vault will be installed on"
  type        = string
}
