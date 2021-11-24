# ipfs-connector

## Setup
```sh
# install faas-cli
brew install faas-cli

# install multipass
brew install multipass

# clone the repo
git clone git@github.com:clstb/ipfs-connector.git && cd ./ipfs-connector

# add your ssh key
vi ./cloud-config.txt

# launch the vm
multipass launch --cloud-init cloud-config.txt  --name faasd


# get the vm IP
export IP=$(multipass info faasd --format json| jq -r '.info.faasd.ipv4[0]')

# set the gateway for faas-cli
export OPENFAAS_URL=http://$IP:8080

# get the gateway password
ssh ubuntu@$IP "sudo cat /var/lib/faasd/secrets/basic-auth-password" > basic-auth-password

# login with faas-cli
cat basic-auth-password | faas-cli login -s

# deploy an annotated function
faas-cli deploy --name echo --image ghcr.io/openfaas/alpine:latest \
        --fprocess=cat \
        --annotation topic="test-topic"

# run the connector
go run main.go run \
	--gateway-password $(cat basic-auth-password) \
	--gateway-url http://$IP:8080 \
	--topics test-topic

# run the gossiper
go run main.go gossiper --topics test-topic
```
