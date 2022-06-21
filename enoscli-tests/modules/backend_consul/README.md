# enos-consul-cluster
The consul_cluster module is a module that creates a Consul cluster using the “aws” and “enos” providers.

# Release Workflow:
This repo uses the GitHub Actions workflow for CI/CD. Terraform fmt, plan, apply, and destroy is run on each PR.
`terraform-enos-aws-consul` is released as a private module in the Terraform Cloud `hashicorp-qti` org.
We use the https://github.com/marketplace/actions/tag-release-on-push-action?version=v0.18.0 GitHub action to
release the module based on the PR labels.

By default there is no release on merge of the PR. Following actions occur on merge(push to `main`) based on the PR label.
## For PR with label:
  * `release:patch` will create a patch release
  * `release:minor` will create a minor release
  * `release:major` will create a major release
