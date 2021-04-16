variable "artifactory_username" {
  type = string
}

variable "artifactory_token" {
  type = string
}

variable "artifactory_host" {
  type = string
}

variable "artifactory_repo" {
  type = string
}

variable "artifactory_path" {
  type = string
}

variable "artifactory_name" {
  type = string
}

variable "artifactory_properties" {
  type = map(string)
  default = {}
}
