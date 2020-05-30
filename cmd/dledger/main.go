package main

import (
    "../../pkg/hashgraph"
    "bufio"
    "fmt"
    "net"
    "os"
    "strings"
)

const (
    DefaultPort = "8080" // Use this default port if it isn't specified via command line arguments
)

type Transaction struct {
    senderAddress   string  // ip:port of sender
    receiverAddress string  // ip:port of receiver
    amount          float64 // amount
}

func main() {
    port := DefaultPort
    if len(os.Args) > 1 {
        port = os.Args[1]
    }
    localIPAddress := getLocalAddress()

    myAddress := localIPAddress + ":" + port
    peerAddresses := readPeerAddresses("peers.txt", localIPAddress)

    _, ok := peerAddresses[myAddress]

    if !ok {
        panic("Peers file does not include my address: " + myAddress)
    }

    fmt.Print(peerAddresses)

    myNode := hashgraph.Node{
        Address:   "",
        Hashgraph: nil,
    }

    go hashgraphMain(myNode)
}

func hashgraphMain(node hashgraph.Node) {
    for {
        // TODO: select a node at random
        // TODO: sync all events with that node
        // TODO: create a new event
        node.DivideRounds()
        node.DecideFame()
        node.FindOrder()
    }
}

func readPeerAddresses(path string, localIpAddr string) map[string]string {
    file, err := os.Open(path)
    handleError(err)
    defer func() {
        handleError(file.Close())
    }()

    // addr to name map
    peers := make(map[string]string)
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        addrName := strings.Split(scanner.Text(), " ")
        peers[strings.Replace(addrName[0], "localhost", localIpAddr, 1)] = addrName[1]
    }
    return peers
}

// returns the local address of this device
func getLocalAddress() string {
    conn, err := net.Dial("udp", "eng.ku.edu.tr:80")
    handleError(err)
    defer func() {
        handleError(conn.Close())
    }()
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP.String()
}

func handleError(e error) {
    if e != nil {
        panic(e)
    }
}
