![Test](https://github.com/hashicorp-forge/terraform-provider-enos/actions/workflows/test.yml/badge.svg)
![Release](https://github.com/hashicorp-forge/terraform-provider-enos/actions/workflows/release.yml/badge.svg)

# terraform-provider-enos

A terraform provider that provides resouces for powering Software Quality as Code by writing
Terraform-based quality requirement scenarios using a composable, modular, and declarative language.

It is intended to be use in conjunction with the [Enos CLI](https://github.com/hashicorp/enos) and
provide the resources necessary to use Terraform as Enos's execution engine.

- [Installing the provider](#installing-the-provider)
  - [Requirements](#requirements)
  - [From the Terraform registry](#from-the-terraform-registry)
- [Developing the provider](#developing-the-provider)
  - [Build from source](#build-from-source)
    - [Flight control](#flight-control)
      - [Commands](#commands)
        - [Download](#download)
        - [Unzip](#unzip)
    - [Remote flight](#remote-flight)
  - [Creating new sources](#creating-new-sources)
  - [Releasing a new version](#releasing-a-new-version)

# Installing the provider

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.2.0

## From the Terraform registry

Install the released version of the provider from the Terraform registry by following the instructions in the [Terraform Registry](https://registry.terraform.io/providers/hashicorp-forge/enos/latest)

```hcl
terraform {
  required_providers {
    enos = {
      source = "hashicorp-forge/enos"
    }
  }
}

provider "enos" {
  # ...
}
```

# Developing the provider

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.2.0
- [Go](https://golang.org/doc/install) >= 1.22

## Build from source

For local development, first you will need to build [flight-control](#flight-control). 

* If you're _not_ on macOS, make sure `upx` installed with your package manager. We use macOS to
  pack some embedded [flight-control](#flight-control) binaries. We don't need this on macOS because
  `upx` has been [removed from Homebrew](https://github.com/upx/upx/issues/612) while they sort out
  macOS code signing shenanigans.

* Run `make flight-control install` in the root of this repository. This will build and pack
  the `enos-flight-control` binaries, build a new `terraform-provider-enos` binary and install it into
  your local Terraform provider cache.

## Flight control
Enos resources that take require a `transport` attribute to be configured work by executing remote
commands on a target resources. Often it's resonably safe to assume that the remote target will
provide some common POSIX commands for common tasks, however, there are some targets or operations
where there is no common POSIX utility we can rely on, such as making remote HTTP requests, unziping
archives, or executing against a minimal container. While utilities that can provide those functions
might be accessible via a package manager of some sort, installing global utlities and dealing with
platform specific package managers can become a serious burden.

Rather than cargo cult brittle and complex script to manage various package managers, our solution
to this problem is to bundle common operations into a binary called `enos-flight-control`. As part
of our build pipeline we build this utility for every platform and architecture that we support and
embed it into the Terraform plugin. During runtime the provider resources can install it on the
remote targets and then call into it when we need advanced operations.

### Commands

#### Download

The download command downloads a file from a given URL and verify the content SHA and send HTTP
requests. It's sort of a Kirkland Signature version of `curl` or `wget`.

`enos-flight-control download --url https://some/remote/file.txt --destination /local/path/file.txt --mode 0755 --timeout 5m --sha256 02b3...`

*Flags*
- `auth-user` The username to use for basic auth|
- `auth-password` The password to use for basic auth|
- `destination` The destination location where the file will be written|
- `exit-with-status-code` On failure, exit with the HTTP status code returned. Note that status codes over 256 are not supported|
- `mode` The desired file permissions of the downloaded file|
- `replace` Replace the destination file if it exists|
- `sha256` The expected SHA256 sum of the file to be downloaded. When provided we'll assert that the resulting file matches the SHA or will raise an error|
- `stdout` Write the output to stdout|
- `timeout` The maximum allowable time for the download operation|
- `url` The URL of the remote resource to download|

*NOTE* one of `--destination` or `--stdout` is required.

#### Unzip

The unzip command unzips a zip archive.

`enos-flight-control unzip --source /some/file.zip --destination /some/directory --create true`

*Flags*
- `source` The path to the source Zip archive|
- `destination` The destination directory where the expanded files will be written|
- `mode` The desired file permissions of the expanded archive files|
- `create-destination` Whether or not create the destination directory if does not exist|
- `destination-mode` The file mode for the destination directory if it is to be created|
- `replace` Replace any existing destination file if they already exist|

## Remote flight

The `remoteflight` package is a library where many common operations that need to be performed over
a transport are located. The include installing `enos-flight-control` on a target machines.

## Releasing a new version
Triggering a new release requires executing the `release` workflow and providing the Git SHA you
wish to release. It will automatically use the version present on that SHA as the release tag. The
version at that SHA must not already exist as a tag.
