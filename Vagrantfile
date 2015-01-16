VAGRANTFILE_API_VERSION = "2"

vm_ip = "172.16.0.3" # arbitrary private IP
pkgs = "lxc-docker build-essential ethtool libpcap-dev git mercurial"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
  config.vm.box = "phusion/ubuntu-14.04-amd64"
  config.vm.box_url = "https://oss-binaries.phusionpassenger.com/vagrant/boxes/latest/ubuntu-14.04-amd64-vbox.box"

  config.vm.network "private_network", ip: vm_ip

  config.vm.synced_folder "./", "/home/vagrant/src/github.com/zettio/weave"

  config.vm.provision :shell, :inline => "sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9"
  config.vm.provision :shell, :inline => "echo deb https://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list"

  pkg_cmd = "export DEBIAN_FRONTEND=noninteractive; " \
    "apt-get update -qq; " \
    "apt-get install -qq --no-install-recommends #{pkgs}"

  install_go_toolchain = "curl -s https://storage.googleapis.com/golang/go1.4.linux-amd64.tar.gz | tar xz -C /usr/local"

  config.vm.provision :shell, :inline => pkg_cmd
  config.vm.provision :shell, :inline => install_go_toolchain
  config.vm.provision :shell, :inline => "usermod -a -G docker vagrant; "
  config.vm.provision :shell, :inline => "echo export GOPATH=/home/vagrant >> /home/vagrant/.bashrc"
  config.vm.provision :shell, :inline => "echo export PATH=/usr/local/go/bin:${PATH} >> /home/vagrant/.bashrc"
  config.vm.provision :shell, :inline => "chown -R vagrant:vagrant /home/vagrant/src"
  config.vm.provision :shell, :inline => "/usr/local/go/bin/go clean -i net; /usr/local/go/bin/go install -tags netgo std"

end

begin
  load 'Vagrantfile.local'
rescue LoadError
end