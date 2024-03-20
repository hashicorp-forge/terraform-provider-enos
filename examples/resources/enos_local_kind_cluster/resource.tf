resource "enos_local_kind_cluster" "my_cluster" {
  name            = "my_cluster"
  kubeconfig_path = abspath(joinpath(path.root, "kubeconfig_my_cluster"))
}
