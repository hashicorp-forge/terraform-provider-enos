#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -e

fail() {
  echo "$1" 1>&2
  exit 1
}

# Test for existence of min
if ! id enosmin; then
  fail "enosmin user does not exist"
fi

# Test for all attributes on all
if ! enosallstr=$(getent passwd enosall); then
  fail "enosall user does not exist"
fi

IFS=':' read -r -a parts <<< "$enosallstr"

if [[ "${parts[2]}" != "900" ]]; then
  fail "expected enosall uid to be 900, got ${parts[2]}"
fi

if [[ "${parts[3]}" != "900" ]]; then
  fail "expected enosall gid to be 900, got ${parts[3]}"
fi

if [[ "${parts[5]}" != "/home/enosall" ]]; then
  fail "expected enosall home directory to be /home/enosall got ${parts[5]}"
fi

if [[ "${parts[6]}" != "/bin/false" ]]; then
  fail "expected enosall shell to be /bin/false got ${parts[6]}"
fi
