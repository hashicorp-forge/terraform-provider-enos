#!/usr/bin/env bash
# Copyright IBM Corp. 2016, 2025
# SPDX-License-Identifier: MPL-2.0

set -e

fail() {
  echo "$1" 1>&2
  exit 1
}

[[ -z "${PACKAGE_MANAGER}" ]] && fail "PACKAGE_MANAGER env variable has not been set"
[[ -z "${RETRY_INTERVAL}" ]] && fail "RETRY_INTERVAL env variable has not been set"
[[ -z "${TIMEOUT_SECONDS}" ]] && fail "TIMEOUT_SECONDS env variable has not been set"

# Synchronize our repositories so that futher installation steps are working with updated cache
# and repo metadata.
synchronize_repos() {
  case $PACKAGE_MANAGER in
    apt)
      sudo apt update
      ;;
    dnf)
      sudo dnf makecache
      ;;
    yum)
      sudo yum makecache
      ;;
    *)
      return 0
      ;;
  esac
}

# Function to check cloud-init status and retry on failure
# Before we start to modify repositories and install packages we'll wait for cloud-init to finish
# so it doesn't race with any of our package installations.
# We run as sudo because Amazon Linux 2 throws Python 2.7 errors when running `cloud-init status` as
# non-root user (known bug).
wait_for_cloud_init() {
  if output=$(sudo cloud-init status --wait); then
    return 0
  else
    res=$?
    case $res in
      2)
        {
          echo "WARNING: cloud-init did not complete successfully but recovered."
          echo "Exit code: $res"
          echo "Output: $output"
          echo "Here are the logs for the failure:"
          cat /var/log/cloud-init-*
        } 1>&2
        return 0
        ;;
      *)
        {
          echo "cloud-init did not complete successfully."
          echo "Exit code: $res"
          echo "Output: $output"
          echo "Here are the logs for the failure:"
          cat /var/log/cloud-init-*
        } 1>&2
        return 1
        ;;
    esac
  fi
}

# Wait for cloud-init if it exists
type cloud-init && wait_for_cloud_init

# Synchronizing repos
begin_time=$(date +%s)
end_time=$((begin_time + TIMEOUT_SECONDS))
while [ "$(date +%s)" -lt "$end_time" ]; do
  if synchronize_repos; then
    exit 0
  fi

  sleep "$RETRY_INTERVAL"
done

fail "Timed out waiting for distro repos to be set up"
