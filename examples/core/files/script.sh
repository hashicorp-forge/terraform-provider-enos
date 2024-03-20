#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


# echo to both STDOUT and STDERR
echo "${HELLO_WORLD}" | tee >(cat >&2)
