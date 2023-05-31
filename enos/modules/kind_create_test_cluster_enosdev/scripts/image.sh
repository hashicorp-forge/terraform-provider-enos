#!/usr/bin/env bash

docker pull alpine:3.16.0
docker tag alpine:3.16.0 "${IMAGE_NAME}:${IMAGE_TAG}"
