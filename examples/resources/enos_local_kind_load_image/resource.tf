# Load a remote image into the cluster
resource "enos_local_kind_load_image" "vault" {
  cluster_name = "my_cluster"
  image        = "vault"
  tag          = "1.15.5"
}

# Load an image archive into the cluster
resource "enos_local_kind_load_image" "vault" {
  cluster_name = "my_cluster"
  image        = "1.16.0-rc1"
  tag          = "1.16.0-rc1"
  archive      = "/path/to/docker-vault-1.16.0-rc1.tar"
}
