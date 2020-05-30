package main

import (
    "../../pkg/hashgraph"
    "bufio"
    "fmt"
    "math/rand"
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
    // Read peer address to name map from file
    peerAddressMap := readPeerAddresses("peers.txt", localIPAddress)

    // Validate your own address is on the peers file
    _, ok := peerAddressMap[myAddress]
    if !ok {
        panic("Peers file does not include my address: " + myAddress)
    }

    // Copy peer addresses to a slice to get random addresses for gossiping
    peerAddresses := make([]string, len(peerAddressMap))
    i := 0
    for k := range peerAddressMap {
        peerAddresses[i] = k
        i++
    }

    initialHashgraph := make(map[string][]hashgraph.Event, len(peerAddressMap))
    // TODO: create initial event
    initialEvent := hashgraph.Event{
        Signature:       "",
        SelfParentHash:  "",
        OtherParentHash: "",
        Timestamp:       0,
        Transactions:    nil,
    }

    for addr := range peerAddressMap {
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

    checkPeersForStart(peerAddresses, myAddress)

    go hashgraphMain(myNode, peerAddresses)

    for {
        continue
    }

}

func hashgraphMain(node hashgraph.Node, peerAddresses []string) {
    for {
        randomPeer := peerAddresses[rand.Intn(len(peerAddresses))]
        knownEventNums := make(map[string]int, len(node.Hashgraph))

        for addr := range node.Hashgraph {
            knownEventNums[addr] = len(node.Hashgraph[addr])
        }

        peerRpcConnection, err := rpc.Dial("tcp", randomPeer)
        handleError(err)
        numEventsToSend := make(map[string]int, len(node.Hashgraph))
        _ = peerRpcConnection.Call("Node.GetNumberOfMissingEvents", knownEventNums, &numEventsToSend)
        _ = peerRpcConnection.Close()

        fmt.Printf("%+x \n", numEventsToSend)


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

func checkPeersForStart(peerAddresses []string, myAddress string) {
    fmt.Println("Welcome to Decentralized Ledger, please wait until all other peers are available.")
    peerAvailable := make([]bool, len(peerAddresses))

    remainingPeers := len(peerAddresses)
    for remainingPeers > 0 {
        for index, isAlreadyResponded := range peerAvailable {
            // we have already reached this peer
            if isAlreadyResponded {
                continue
            }

            // this peer is us :)
            if peerAddresses[index] == myAddress {
                peerAvailable[index] = true
                remainingPeers--
                continue
            }

            rpcConnection, err := rpc.Dial("tcp", peerAddresses[index])
            if err != nil {
                continue
            } else {
                _ = rpcConnection.Close()
                peerAvailable[index] = true
                remainingPeers--
            }
        }
    }
    fmt.Println("All peers are available, you can start sending messages")
}