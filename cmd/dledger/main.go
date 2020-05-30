package main

import (
    "../../pkg/hashgraph"
    "bufio"
    "fmt"
    "net"
    "net/rpc"
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


    initialHashgraph := make(map[string][]hashgraph.Event, len(peerAddresses))
    // TODO: create initial event
    initialEvent := hashgraph.Event{
        Signature:       "",
        SelfParentHash:  "",
        OtherParentHash: "",
        Timestamp:       0,
        Transactions:    nil,
    }

    for addr := range peerAddresses {
        initialHashgraph[addr] = make([]hashgraph.Event, 0)
    }

    initialHashgraph[myAddress] = append(initialHashgraph[myAddress], initialEvent)

    myNode := hashgraph.Node{
        Address:   myAddress,
        Hashgraph: initialHashgraph,
    }

    _ = rpc.Register(&myNode)
    tcpAddr, _ := net.ResolveTCPAddr("tcp", myAddress)
    listener, _ := net.ListenTCP("tcp", tcpAddr)
    go listenForRPCConnections(listener)

    // TODO: wait until all nodes are online

    go hashgraphMain(myNode)

    // TODO: our application I/O logic

    fmt.Printf("%s\n", peerAddresses)

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

func listenForRPCConnections(listener *net.TCPListener) {
    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        rpc.ServeConn(conn)
    }
}
