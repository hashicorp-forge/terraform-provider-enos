#!/bin/bash

# echo to both STDOUT and STDERR
echo "${HELLO_WORLD}" | tee >(cat >&2)
