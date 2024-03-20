# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_providers {
    enos = {
      version = ">= 0.1"
      source  = "hashicorp.com/qti/enos"
    }
  }
}

resource "enos_local_exec" "foo" {
  inline = [
    "mkdir -p /tmp/local-foo",
    "touch /tmp/local-foo/foo",
  ]

  scripts = ["${path.module}/foo.sh"]

  content = templatefile("${path.module}/foo.sh.tpl", {
    stderr = "foo1"
  })
}

resource "enos_local_exec" "bar" {
  depends_on = [enos_local_exec.foo]
  inline = [
    "mkdir -p /tmp/local-bar",
    "touch /tmp/local-bar/bar",
    "echo '${enos_local_exec.foo.stderr}' > /tmp/local-bar/foo-stderr"
  ]

  scripts = ["${path.module}/foo.sh"]

  content = templatefile("${path.module}/foo.sh.tpl", {
    stderr = "bar"
  })
}

resource "enos_local_exec" "baz" {
  depends_on = [enos_local_exec.bar]
  inline     = ["true"]
}

output "blank" {
  value = resource.enos_local_exec.baz.stdout
}
