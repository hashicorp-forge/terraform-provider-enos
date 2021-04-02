# mirror

This repository contains the Terraform used to maintain S3 bucket policy of
the network mirror used to distribute the enos-provider.

In order to use this you'll need to be a member of the QTI TFC organization.

You'll want to create some an auto tfvars file to specify the inputs during
Terraform application, eg:

```hcl
bucket = "enos-provider"
allowed_ips = [
  "192.168.0.1", # Note, ensure that we're appending the existing list
]
aws_region = "us-west-2"
```

For publishing to the mirror, see the guide in the main README.
