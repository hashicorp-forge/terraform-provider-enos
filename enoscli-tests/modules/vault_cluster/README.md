# enos-vault-cluster
The vault_cluster module is a module that creates a Vault cluster using the “aws” and “enos” providers.

# Release Workflow:
This repo uses the GitHub Actions workflow for CI/CD. Terraform plan/apply/destroy are run on each PR in
the `quality-team-enos-ci` AWS account.

The `terraform-enos-aws-vault` module is released as a private module in the Terraform Cloud `hashicorp-qti` org.
We use the [release-on-push](https://github.com/marketplace/actions/tag-release-on-push-action?version=v0.18.0) GitHub action to
release the module based on the PR labels.

By default there is no release on merge of the PR.  Based on the PR label following  actions occur on merge(push to `main`)
## For PR with label:
  * `release:patch` will create a patch release
  * `release:minor` will create a minor release
  * `release:major` will create a major release
