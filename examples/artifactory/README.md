# artifactory

This example module uses demonstrates the enos utilizing artifactory.

To used this module you'll need to provide the required credentials and
search criteria in order to search for a valid binary.

For example, add the follow to a `terraform.tfvars` file:


```hcl
artifactory_username   = "<your hashicorp email address>"
artifactory_token      = "<your artifactory token>"
artifactory_host       = "https://artifactory.hashicorp.engineering/artifactory"
artifactory_repo       = "hashicorp-packagespec-buildcache-local*"
artifactory_path       = "cache-v1/vault-enterprise/*"
artifactory_name       = "*.zip"
artifactory_properties = {
  "EDITION"         = "ent"
  "GOARCH"          = "amd64"
  "GOOS"            = "linux"
  "artifactType"    = "package"
  "productRevision" = "f45845666b4e552bfc8ca775834a3ef6fc097fe0"
  "productVersion"  = "1.7.0"
}
```

Assuming the artifacts that match the example havent't been deleted, the datasource
should return multiple URLs to a matching artifact.

Note: it will return many results because we use aliases. Any of the URLs should
do as they all refer to the same artifact.

You can also use _any_ artifactory property to narrow the search.
