# Hashgraph in Go
[![DOI](https://zenodo.org/badge/264742438.svg)](https://zenodo.org/badge/latestdoi/264742438)

This is the course project for COMP515 - Distributed Systems. As a team of 2: [Erhan Tezcan](https://github.com/erhant) & [Ahmet Uysal](https://github.com/ahmetuysal), we implemented hashgraph in Golang.

## Disclaimer
Hashgraph is a patented algorithm which is developed by Leemon Baird, the co-founder and CTO of Swirlds, in 2016. This project is developed solely for education purposes to better understand how Hashgraph works. You find the original papers we used for our implementation in our [report](report.pdf).

## How to run
There are two applications located under [`cmd`](cmd) folder. 

- [`dledger`](cmd/dledger) contains a distributed ledger application that is built upon Hashgraph algorithm. <br>
You can run `dledger` by `$ go run main.go PORT_NUMBER`. Note that this application retrieves the peer information from [`peers.txt`](cmd/dledger/peers.txt)
- [`ui`](cmd/ui) contains a visualization application that shows the current state of Hashgraph in realtime. `ui` is built using `go-astilectron`. You can check [`go-astilectron` repository](https://github.com/asticode/go-astilectron) to get more information about installation and running.
