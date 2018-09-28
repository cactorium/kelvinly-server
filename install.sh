#!/bin/bash

sudo add-apt-repository ppa:gophers/archive
sudo apt-get update
sudo apt-get -y install golang-1.10-go
sudo apt-get -y install iptables-persistent

mkdir -p ~/go

cat "export PATH=$PATH:/usr/lib/go-1.10/bin" >> ~/.bashrc
cat "export GOPATH=~/go" >> ~/.bashrc

source ~/.bashrc
go get -u gopkg.in/russross/blackfriday.v2
go get -u https://github.com/shurcooL/github_flavored_markdown
go get -u github.com/sevlyar/go-daemon
