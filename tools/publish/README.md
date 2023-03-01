# publish

The `publish` go CLI utility is used to publish to TFC private provider registry.

# tfc upload command

The `tfc upload` sub-command is how we take the enos-provider Terraform plugin binaries, create a signing file, and publish them to a private registry in a TFC org
It takes artifacts from the local source directory, creates and signs the SHASUMS file, and publishes the release files to private provider's registry in `hashicorp-qti` org in Terraform Cloud. The default GPG Identity is QTI team's email address `team-secure-quality@hashicorp.com` and its generated key `5D67D7B072C16294` is uploaded to `hashicorp-qti` TFC org.  This allows artifacts signed using this key to be published to private providers in `hashicorp-qti`. `TFC_PUBLISH_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc upload --dist [DIR] --gpg-key-id [GPG SIGNING KEY] --binary-name [BINARY NAME] --provider-name [PROVIDER] --rename-binary [RENAMED BINARY] --org [TFC ORG NAME] --token [TFC_PUBLISH_TOKEN] [flags]
```

## example
To publish the artifacts for enos-provider version `0.1.20` from local directory path `./dist` to
private provider registry in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc upload --dist ./dist --gpg-key-id 5860AD9288 --org hashicorp-qti --token $TFC_PUBLISH_TOKEN
```

# tfc download command

The `tfc download` sub-command downloads the enos-provider Terraform plugin binaries from a private registry in TFC org for the given version. It downloads the binaries to the provided download directory path, using the `TFC_PUBLISH_TOKEN` to authenticate to TFC. `TFC_PUBLISH_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc download --download-dir [DIR] --binary-name [BINARY NAME] --provider-name [PROVIDER] --provider-version [VERSION] --org [TFC ORG NAME] --token [TFC_PUBLISH_TOKEN] [flags]
```

## example
To download the artifacts for enos-provider version `0.1.20` to local directory path `./enos-downloads` from private provider registry `enosdev` in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc download --download-dir ./enos-downloads --provider-version 0.1.20 --org hashicorp-qti --token $TFC_PUBLISH_TOKEN
```

# tfc promote command

The `tfc promote` sub-command promotes the enos-provider Terraform plugin binaries from one private registry to another private registry in TFC org for the given version. It downloads the binaries from dev TFC private provider registry to the provided download directory path, extracts them to the provided output directory, creates a zip archive and SHASUMS file, and uploads them (to the prod TFC private provider registry) using the `TFC_PUBLISH_TOKEN` to authenticate to TFC. `TFC_PUBLISH_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc promote --src-binary-name [DEV BINARY NAME] --src-provider-name [DEV PROVIDER NAME] --dest-binary-name [PROD BINARY NAME] --dest-provider-name [PROD PROVIDER NAME] --provider-version [VERSION] --org [TFC ORG NAME] --token [TFC_PUBLISH_TOKEN] [flags]
```

## example
To promote the artifacts for enos-provider version `0.2.1` from private provider registry `enosdev` to private provider registry `enos` in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc promote --src-binary-name terraform-provider-enosdev  --src-provider-name enosdev --dest-binary-name terraform-provider-enos --dest-provider-name enos --provider-version 0.2.1 --token $TFC_PUBLISH_TOKEN
```
