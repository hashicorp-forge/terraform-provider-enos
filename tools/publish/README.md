# publish

The `publish` go CLI utility is used to manage S3 backed Terraform Plugin `network_mirror`'s.
As Terraform Cloud currently does not allow private Terraform plugin hosting, the only way to easily
distribute a private Terraform provider is through a network_mirror.
This CLI allows us to distribute the provider without running an internal private provider registry.

# s3 upload command

The `s3 upload` sub-command is how we take the enos-provider Terraform plugin binaries, package them into
compatible Terraform Plugin archives, publish them to an S3 network mirror, and generate the required mirror metadata.
It works by pointing to the `dist` (output directory) of `goreleaser`, along with the S3 bucket and optionally
the provider identifier and the plugin name.

## command syntax
```sh
    go run ./tools/publish/cmd s3 upload --dist [DIR] --bucket [S3 BUCKETNAME] [flags]
```

## example
To publish the built artifacts to a remote `enos-provider-current` S3 mirror, run from the root of the repository:
```sh
CI=true make release
go run ./tools/publish/cmd s3 upload --dist ./dist --bucket enos-provider-current
```

# s3 copy command

As we grow we need to be able to publish `enos-provider` versions to a current bucket and, when we feel comfortable, s3 copy it to a stable bucket which downstream tooling can rely upon. We do this by copying a release version from the `current` bucket to the `stable` bucket. The `s3 copy` sub-command copies the required release artifacts from the source bucket to the destination bucket and then creates a new index in the destination bucket that includes the new release.

## command syntax
```sh
    go run ./tools/publish/cmd s3 copy --version [ARTIFACT_VERSION] --src-bucket [S3 BUCKETNAME] --dest-bucket [S3 BUCKETNAME] [flags]
```

## example
To s3 copy the artifacts for enos-provider version `0.0.3` from current remote S3 bucket `enos-provider-current` to
stable remote S3 bucket `enos-provider-stable`, run from the root of the repository:
```sh
go run ./tools/publish/cmd s3 copy --version 0.0.3 --src-bucket enos-provider-current --dest-bucket enos-provider-stable
```

# tfc upload command

The `tfc upload` sub-command is how we take the enos-provider Terraform plugin binaries, create a signing file, and publish them to a private registry in a TFC org
It takes artifacts from the local source directory, creates and signs the SHASUMS file, and publishes the release files to private provider's registry in `hashicorp-qti` org in Terraform Cloud. The default GPG Identity is QTI team's email address `team-secure-quality@hashicorp.com` and its generated key `5D67D7B072C16294` is uploaded to `hashicorp-qti` TFC org.  This allows artifacts signed using this key to be published to private providers in `hashicorp-qti`. `TFC_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc upload --dist [DIR] --gpg-key-id [GPG SIGNING KEY] --binary-name [BINARY NAME] --provider-name [PROVIDER] --rename-binary [RENAMED BINARY] --org [TFC ORG NAME] --token [TFC_TOKEN] [flags]
```

## example
To publish the artifacts for enos-provider version `0.1.20` from local directory path `./dist` to
private provider registry in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc upload --dist ./dist --gpg-key-id 5860AD9288 --org hashicorp-qti --token $TFC_TOKEN
```

# tfc download command

The `tfc download` sub-command downloads the enos-provider Terraform plugin binaries from a private registry in TFC org for the given version. It downloads the binaries to the provided download directory path, using the `TFC_TOKEN` to authenticate to TFC. `TFC_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc download --download-dir [DIR] --binary-name [BINARY NAME] --provider-name [PROVIDER] --provider-version [VERSION] --org [TFC ORG NAME] --token [TFC_TOKEN] [flags]
```

## example
To download the artifacts for enos-provider version `0.1.20` to local directory path `./enos-downloads` from private provider registry `enosdev` in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc download --download-dir ./enos-downloads --provider-version 0.1.20 --org hashicorp-qti --token $TFC_TOKEN
```

# tfc promote command

The `tfc promote` sub-command promotes the enos-provider Terraform plugin binaries from one private registry to another private registry in TFC org for the given version. It downloads the binaries from dev TFC private provider registry to the provided download directory path, extracts them to the provided output directory, creates a zip archive and SHASUMS file, and uploads them (to the prod TFC private provider registry) using the `TFC_TOKEN` to authenticate to TFC. `TFC_TOKEN` is the authentication token with publish permissions to `hashicorp-qti` org. It can be found in 1password for secure-quality-team.

## command syntax
```sh
    go run ./tools/publish/cmd tfc promote --src-binary-name [DEV BINARY NAME] --src-provider-name [DEV PROVIDER NAME] --dest-binary-name [PROD BINARY NAME] --dest-provider-name [PROD PROVIDER NAME] --provider-version [VERSION] --org [TFC ORG NAME] --token [TFC_TOKEN] [flags]
```

## example
To promote the artifacts for enos-provider version `0.2.1` from private provider registry `enosdev` to private provider registry `enos` in `hashicorp-qti` org, run from the root of the repository:
```sh
go run ./tools/publish/cmd tfc promote --src-binary-name terraform-provider-enosdev  --src-provider-name enosdev --dest-binary-name terraform-provider-enos --dest-provider-name enos --provider-version 0.2.1 --token $TFC_TOKEN
```
