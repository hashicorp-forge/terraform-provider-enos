---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "enos_bundle_install Resource - terraform-provider-enos"
subcategory: ""
description: |-
  The enos_bundle_install resource is capable of installing HashiCorp release bundles, Debian packages,
  or RPM packages, from a local path, releases.hashicorp.com, or from Artifactory directly onto a
  remote node. While it is possible to use to install any debian or RPM packages from Artifactory or
  from a local source, it has been designed for HashiCorp's release workflow.
  While all local artifact, releases.hashicorp.com, and Artifactory methods of install are supported,
  only one can be configured at a time.
---

# enos_bundle_install (Resource)

The `enos_bundle_install` resource is capable of installing HashiCorp release bundles, Debian packages,
or RPM packages, from a local path, releases.hashicorp.com, or from Artifactory directly onto a
remote node. While it is possible to use to install any debian or RPM packages from Artifactory or
from a local source, it has been designed for HashiCorp's release workflow.

While all local artifact, releases.hashicorp.com, and Artifactory methods of install are supported,
only one can be configured at a time.



<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `artifactory` (Object, Sensitive) - `artifactory.username` (String) The Artifactory API username. This will likely be your hashicorp email address
- `artifactory.token` (String) The Artifactory API token. You can sign into Artifactory and generate one
- `artifactory.url` (String) The fully qualified Artifactory item URL. You can use enos_artifactory_item to search for this URL
- `artifactory.sha256` (String) The Artifactory item SHA 256 sum. If present this will be verified on the remote target before the package is installed (see [below for nested schema](#nestedatt--artifactory))
- `destination` (String) The destination directory of the installed binary, eg: /usr/local/bin/. This is required if the artifact is a zip archive and optional when installing RPM or Deb packages
- `getter` (String) The method used to fetch the package
- `installer` (String) The method used to install the package
- `name` (String) The name of the artifact that was installed
- `path` (String) The local path to a zip archive install bundle.
- `release` (Object) - `release.product` (String) The product name that you wish to install, eg: 'vault' or 'consul'
- `release.version` (String) The version of the product that you wish to install. Use the full semver version ('2.1.3' or 'latest')
- `release.edition` (String) The edition of the product that you wish to install. Eg: 'ce', 'ent', 'ent.hsm', 'ent.hsm.fips1403', etc. (see [below for nested schema](#nestedatt--release))
- `transport` (Dynamic) - `transport.ssh` (Object) the ssh transport configuration
- `transport.ssh.user` (String) the ssh login user|string
- `transport.ssh.host` (String) the remote host to access
- `transport.ssh.private_key` (String) the private key as a string
- `transport.ssh.private_key_path` (String) the path to a private key file
- `transport.ssh.passphrase` (String) a passphrase if the private key requires one
- `transport.ssh.passphrase_path` (String) a path to a file with the passphrase for the private key

### Read-Only

- `id` (String) The resource identifier is always static

<a id="nestedatt--artifactory"></a>
### Nested Schema for `artifactory`

Optional:

- `sha256` (String)
- `token` (String)
- `url` (String)
- `username` (String)


<a id="nestedatt--release"></a>
### Nested Schema for `release`

Optional:

- `edition` (String)
- `product` (String)
- `version` (String)
