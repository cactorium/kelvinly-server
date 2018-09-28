#!/bin/bash

sudo add-apt-repository ppa:gophers/archive
sudo apt-get update
sudo apt-get -y install golang-1.10-go
sudo apt-get -y install iptables-persistent

mkdir -p ~/go

echo "export PATH=$PATH:/usr/lib/go-1.10/bin" >> ~/.bashrc
echo "export GOPATH=~/go" >> ~/.bashrc

sudo cp rules.v4 /etc/iptables/rules.v4
sudo cp kelvinly-server.service /etc/systemd/system

source ~/.bashrc
go get -u gopkg.in/russross/blackfriday.v2
go get -u github.com/shurcooL/github_flavored_markdown
go get -u github.com/sevlyar/go-daemon
