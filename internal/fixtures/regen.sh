#!/bin/bash -e

openssl genrsa -out ssh.pem 4096
openssl rsa -in ssh.pem -pubout -out ssh.pub
openssl genrsa -aes256 -passout file:passphrase.txt -out ssh_pass.pem 4096
openssl rsa -in ssh_pass.pem -passin file:passphrase.txt -pubout -out ssh_pass.pub
