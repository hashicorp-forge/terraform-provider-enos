#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


docker pull alpine:3.16.0
docker tag alpine:3.16.0 "${IMAGE_NAME}:${IMAGE_TAG}"
