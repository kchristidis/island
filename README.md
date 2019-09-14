# island

A simulator for blockchain-based local energy markets.

## Dependencies

* go
* dep
* vagrant
* vagrant-disksize
* virtualbox

We suggest using the [Homebrew package manager](https://brew.sh/). Then you can pick up the latest versions of all of the above like so:

```bash
brew install go dep
brew cask install virtualbox vagrant
vagrant plugin install vagrant-disksize
```

## Setup

Clone the forked `fabric` repo and the `island` repo:

```bash
git clone git@github.com:kchristidis/fabric.git
git clone git@github.com:kchristidis/island.git
```

We assume that `fabric` lives in `$GOPATH/src/github.com/hyperledger/fabric` and `island` in `$GOPATH/src/github.com/kchristidis/island`. If that is not the case, you'll have to [edit the Vagrantfile accordingly](https://github.com/kchristidis/fabric/blob/901b8db0bb46a90cf9eb9fbb2e7cbd2fc9fcada3/devenv/Vagrantfile#L19..L20).

Then cd into the `island` directory and:

```bash
dep ensure
```

cd into the `trace` directory within `island`, and download `04-final-trace-2013.csv` ([link](https://drive.google.com/open?id=1snADVFVuzFOE52M9ynVvKXVvif5AJakV)) there.

Finally, cd into the `fabric` directory and:

```bash
git fetch --all
git checkout island
```

## Daily operation

cd into the `fabric` directory:

```bash
cd devenv
vagrant up # replace with 'vagrant ssh' in subsequent calls when the VM is up already
```

When inside the VM:

```bash
make all
```

When the simulation is over, the metrics are captured in the `output` folder in the `island` repo.

## Credits

This repo began its life as a fork of the [heroes-service repo](https://github.com/chainHero/heroes-service). Experiments 2-3 make use of the composite keys iteration as demonstrated in the [high-throughput Fabric sample](https://github.com/hyperledger/fabric-samples/blob/ab46e3548c46acf1c541eca71914c20bbe212f6a/high-throughput/README.md).

## Contributing

Contributions are welcome. Fork this repo and submit a pull request.
