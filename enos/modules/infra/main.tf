terraform {
  required_version = ">= 1.1.2"
}

resource "random_pet" "default" {
  separator = "_"
}
