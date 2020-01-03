#!/bin/bash

# Copyright IBM Corp. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0

set -e
set -x

# https://serverfault.com/a/670688
export DEBIAN_FRONTEND=noninteractive

# https://serverfault.com/a/644182
apt-get update -qq

# Install some basic utilities
apt-get install -y build-essential make unzip g++ libtool

# ----------------------------------------------------------------
# Install Docker
# ----------------------------------------------------------------

# Prep apt-get for docker install
apt-get install -y apt-transport-https ca-certificates
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -

# Add docker repository
add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

# Update system
apt-get update -qq

# Install docker
apt-get install -y docker-ce

# Install docker-compose
curl -sL https://github.com/docker/compose/releases/download/1.14.0/docker-compose-`uname -s`-`uname -m` > /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Add vagrant user to the docker group
usermod -a -G docker vagrant

# Test docker
docker run --rm busybox echo All good

# ----------------------------------------------------------------
# Install Go
# ----------------------------------------------------------------
GO_VER=1.13.5
GO_URL=https://dl.google.com/go/go${GO_VER}.linux-amd64.tar.gz

# Set Go environment variables needed by other scripts
export GOPATH="/opt/gopath"
export GOROOT="/opt/go"
PATH=$GOROOT/bin:$GOPATH/bin:$PATH

cat <<EOF >/etc/profile.d/goroot.sh
export GOROOT=$GOROOT
export GOPATH=$GOPATH
export PATH=\$PATH:$GOROOT/bin:$GOPATH/bin
EOF

mkdir -p $GOROOT

curl -sL $GO_URL | (cd $GOROOT && tar --strip-components 1 -xz)

# Ensure permissions are set for GOPATH
sudo chown -R vagrant:vagrant $GOPATH

# ----------------------------------------------------------------
# Misc tasks
# ----------------------------------------------------------------

# Create directory for the DB
sudo mkdir -p /var/hyperledger
sudo chown -R vagrant:vagrant /var/hyperledger

# Update limits.conf to increase nofiles for LevelDB and network connections
sudo cp $GOPATH/src/github.com/kchristidis/island/fixtures/vagrant/limits.conf /etc/security/limits.conf

# Set our shell prompt to something less ugly
cat <<EOF >> /home/vagrant/.bashrc
cd $GOPATH/src/github.com/kchristidis/island/
PS1="\u@island:$(git rev-parse --short HEAD):\w$ "
EOF