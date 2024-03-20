# Find the pods matching the labels in the given cluster. If your pod is created at apply time you
# might have to set a depends_on manually if Terraform doesn't know about an implicit relationship.
data "enos_kubernetes_pods" "test" {
  kubeconfig_base64 = enos_local_kind_cluster.test.kubeconfig_base64
  context_name      = enos_local_kind_cluster.test.context_name
  namespace         = helm_release.test.namespace
  label_selectors = [
    "app.kubernetes.io/instance=ci-test",
    "app.kubernetes.io/name=ci-test"
  ]
}
