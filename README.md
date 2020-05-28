# Hashgraph in Go

This is the course project for COMP515 - Distributed Systems. As a team of 2: [Erhan Tezcan](https://github.com/erhant) & [Ahmet Uysal](https://github.com/ahmetuysal), we implemented hashgraph in Golang.

## How to run
Under the `nodes` file, we have `peer.go` which is the main code that we will run using `go run peer.go`. Since we want to do tests in local environment, we have `makepeers_local.sh` script that copies both the source file and the text file to directories, named by their peer number. Then we go to those directories, open a terminal and run the script with it's peer id as a command line argument.