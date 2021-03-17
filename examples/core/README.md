# Core example

This is an example terraform root module that demonstrates the enos provider
against an Ec2 target instance.

By default it runs in us-west-2 region, as such you'll need to have an SSH key
pair in that AWS region or modify the example to a region you'd prefer. You'll
need to supply the keyname to Terraform so that it associates the proper key
when provisioning the target instance.

In the example we demonstrate using provider level configuration for the Enos
transport settings, therefore you'll need to set the ENOS_TRANSPORT_PRIVATE_KEY_PATH
environment variable so that the provider can build an SSH transport to the instance.

For example, run the following commands to run the core example. You'll want
to start the in the root of the git repository, not the `example/core` directory.

```shell
make
export TF_VAR_key_name=<your-key-name>
export ENOS_TRANSPORT_PRIVATE_KEY_PATH=</path/to/your/key.pem>
cd ./examples/core
rm .terraform.lock.hcl # you'll need to do this each time you rebuild the provider
terraform init
terraform plan -out=tf.plan
terraform apply tf.plan
```
