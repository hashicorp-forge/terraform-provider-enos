variable "instance_type" {
  description = "The instance type to provision"
  type        = string
  default     = "t3.micro"
}

variable "tags" {
  description = "The tags to tag the remote host with"
  type        = map(string)
  default     = {}
}
