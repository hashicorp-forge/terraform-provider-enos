# populate-mirror

The `populate-mirror` tool is intended to take the output of `goreleaser` and create
a remote mirror in S3 that Terraform can use to install the provider. This allows
us to distribute the provider without running an internal private provider registry.

To use the tool, run from the root of the repository:
```sh
go run ./tools/populate-mirror -dist ./dist -bucket enos-provider
```
