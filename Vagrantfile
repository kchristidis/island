# Copyright IBM Corp All Rights Reserved
# SPDX-License-Identifier: Apache-2.0

SRCMOUNT = "/opt/gopath/src/github.com/kchristidis/island"

$inline = <<EOF
set -x
cd #{SRCMOUNT}/fixtures/vagrant
./setup.sh
EOF

Vagrant.require_version ">= 1.7.4"
Vagrant.configure('2') do |config|
  config.vm.box = "ubuntu/xenial64"
  config.vm.box_version = ">= 20191108"
  config.vm.synced_folder ".", "#{SRCMOUNT}"
  config.disksize.size = '20GB'
  config.vm.provider :virtualbox do |vb|
    vb.name = "island"
    vb.customize ['modifyvm', :id, '--memory', '4096']
    vb.cpus = 2
  end
  config.vm.provision :shell, inline: $inline
  config.ssh.shell = "bash -c 'BASH_ENV=/etc/profile exec bash'" # https://github.com/hashicorp/vagrant/issues/1673#issuecomment-28288042
end
