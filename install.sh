#!/bin/bash

sudo apt-get update
sudo apt-get -y install golang
sudo apt-get -y install iptables-persistent

mkdir -p ~/go

echo "export PATH=$PATH:/usr/lib/go-1.10/bin" >> ~/.bashrc
echo "export GOPATH=~/go" >> ~/.bashrc

sudo cp rules.v4 /etc/iptables/rules.v4
sudo service netfilter-persistent reload

sudo cp main-server.service /etc/systemd/system
sudo cp gogs.service /etc/systemd/system

source ~/.bashrc
#go get -u gopkg.in/russross/blackfriday.v2
go get -u github.com/shurcooL/github_flavored_markdown
#go get -u github.com/sevlyar/go-daemon

sudo apt-get install certbot python3-certbot-dns-google

echo "cerbot still needs setup, and the servers need to be enabled!"
