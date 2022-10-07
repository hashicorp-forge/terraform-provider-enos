terraform {
  required_version = ">= 0.15.3"
}

provider "helm" {
  kubernetes {
    host = var.host

    client_certificate     = var.client_certificate
    client_key             = var.client_key
    cluster_ca_certificate = var.cluster_ca_certificate
  }
}

resource "helm_release" "ci-test" {
  name  = "ci-test"
  chart = "${path.module}/helm/ci-test"

  namespace        = "ci-test"
  create_namespace = true

  set {
    name  = "replicaCount"
    value = var.replica_count
  }
  set {
    name = "image.repository"
    value = var.repository
  }
  set {
    name = "image.tag"
    value = var.tag
  }

  wait = true
}

variable "host" {
  type        = string
  description = "The hostname (in form of URI) of the Kubernetes API"
}

variable "client_certificate" {
  type        = string
  description = "PEM-encoded client certificate for TLS authentication"
}

variable "client_key" {
  type        = string
  description = "PEM-encoded client certificate key for TLS authentication."
}

variable "cluster_ca_certificate" {
  type        = string
  description = "PEM-encoded root certificates bundle for TLS authentication"
}

variable "replica_count" {
  type        = number
  description = "The number of pods to deploy"
}

variable "repository" {
  type = string
  description = "The docker repository for the image to deploy"
}

variable "tag" {
  type = string
  description = "The tag of the docker image to deploy"
}
