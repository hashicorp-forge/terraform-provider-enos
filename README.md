# enos-provider
A terraform provider for quality infrastructure

# Creating new resources
To ease the burden when creating new resources, we have a scaffolding generator that can take the name of the resource you wish to create and generates the near 400 lines of boilerplate for you. Simply run the following command and then address all the `TODO` statements in your newly generated resource.

From the root directory of this repo, run:
```shell
go run ./tools/create_resource -name <<you_resource_name>>
```
