package dledger

import (
    "bufio"
    "fmt"
    "math/rand"
    "net"
    "net/rpc"
    "os"
    "strings"
    "time"

    "../hashgraph"
    uuid "github.com/satori/go.uuid"
)

const (
    verbose                    = 0                       // 1: prints within RPC, 2: prints within main, off otherwise
    gossipWaitTime             = 4000 * time.Millisecond // the amount of time.sleep milliseconds between each random gossip
    connectionAttemptDelayTime = 100 * time.Millisecond  // the amount of time.sleep milliseconds between each connection attempt
)

//DLedger : Struct for a member of the distributed ledger
type DLedger struct {
    Node           *hashgraph.Node
    MyAddress      string
    PeerAddresses  []string
    PeerAddressMap map[string]string
}

//NewDLedger : Initialize a member in the distributed ledger.
// This is not adding a new member, but rather reading a member from a list and initializing it.
func NewDLedger(port string, peersFilePath string) *DLedger {
    localIPAddress := getLocalAddress()
    myAddress := localIPAddress + ":" + port
    peerAddressMap := readPeerAddresses(peersFilePath, localIPAddress)

    // Assert that your own address is on the peers file
    _, ok := peerAddressMap[myAddress]
    if !ok {
        panic("Peers file does not include my address: " + myAddress)
    }

    // Copy peer addresses to a slice for random access during gossip
    peerAddresses := make([]string, len(peerAddressMap)-1)
    i := 0
    for k := range peerAddressMap {
        if k != myAddress {
            peerAddresses[i] = k // suggestion #1: do not add my own ip:port here, instead of creating another slice without me later
            i++
        }
    }

    // Setup the Hashgraph
    signatureUUID, err := uuid.NewV4()
    handleError(err)
    signature := signatureUUID.String()
    handleError(err)
    initialHashgraph := make(map[string][]*hashgraph.Event, len(peerAddressMap))
    for addr := range peerAddressMap {
        initialHashgraph[addr] = make([]*hashgraph.Event, 0) // We should not know any event other than our own event at the start
    }
    initialEvent := hashgraph.Event{
        Owner:              myAddress,
        Signature:          signature,
        SelfParentHash:     "",
        OtherParentHash:    "",
        Timestamp:          time.Now(),
        Transactions:       nil,
        Round:              1,
        IsWitness:          true, // true because the initial event is the first event of its round
        IsFamous:           false,
        IsFameDecided:      false,
        RoundReceived:      0,
        ConsensusTimestamp: time.Unix(0, 0),
    }
    initialHashgraph[myAddress] = append(initialHashgraph[myAddress], &initialEvent)

    // Initialize the node
    myNode := hashgraph.NewNode(initialHashgraph, myAddress)

    for addr := range myNode.Hashgraph {
        myNode.Witnesses[addr] = make(map[uint32]*hashgraph.Event)
    }
    myNode.Witnesses[initialEvent.Owner][1] = &initialEvent
    myNode.Events[initialEvent.Signature] = &initialEvent
    myNode.FirstRoundOfFameUndecided[initialEvent.Owner] = 1
    myNode.FirstEventOfNotConsensusIndex[initialEvent.Owner] = 0 // index 0 for the initial event

    // Setup the server
    _ = rpc.Register(&myNode)
    tcpAddr, _ := net.ResolveTCPAddr("tcp", myAddress)
    listener, _ := net.ListenTCP("tcp", tcpAddr)
    go listenForRPCConnections(listener)

    return &DLedger{
        Node:           myNode,
        MyAddress:      myAddress,
        PeerAddresses:  peerAddresses,
        PeerAddressMap: peerAddressMap,
    }
}

// Read the node addresses and names, return a map from addresses to names
func readPeerAddresses(path string, localIPAddr string) map[string]string {
    file, err := os.Open(path)
    handleError(err)
    defer func() {
        handleError(file.Close())
    }()

    // Addr to name map
    peers := make(map[string]string)
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        addrName := strings.Split(scanner.Text(), " ")
        peers[strings.Replace(addrName[0], "localhost", localIPAddr, 1)] = addrName[1]
    }
    return peers
}

//Start : Starts the gossip routine in a go routine.
func (dl *DLedger) Start() {
    go gossipRoutine(dl.Node, dl.PeerAddresses)
}

//PerformTransaction : Adds a transaction to the member's buffer.
func (dl *DLedger) PerformTransaction(receiverAddr string, amount float64) {
    dl.Node.RWMutex.Lock()
    dl.Node.TransactionBuffer = append(dl.Node.TransactionBuffer, hashgraph.Transaction{
        SenderAddress:   dl.MyAddress,
        ReceiverAddress: receiverAddr,
        Amount:          amount,
    })
    dl.Node.RWMutex.Unlock()
}

//WaitForPeers : Waits for all members in the member list to be online and responsive.
func (dl *DLedger) WaitForPeers() {
    peerAvailable := make([]bool, len(dl.PeerAddresses))
    remainingPeers := len(dl.PeerAddresses)
    for remainingPeers > 0 {
        for index, hasAlreadyResponded := range peerAvailable {
            // we have already reached this peer
            if hasAlreadyResponded {
                continue
            }

            rpcConnection, err := rpc.Dial("tcp", dl.PeerAddresses[index])
            if err != nil {
                time.Sleep(connectionAttemptDelayTime)
                continue
            } else {
                _ = rpcConnection.Close()
                peerAvailable[index] = true
                remainingPeers--
            }
        }
    }
}

// Infinite loop of gossip routine, each gossip delayed by a constant time.
func gossipRoutine(node *hashgraph.Node, peerAddresses []string) {
    for {
        // Choose a peer
        randomPeer := peerAddresses[rand.Intn(len(peerAddresses))]

        // Calculate how many events I know
        knownEventNums := make(map[string]int, len(node.Hashgraph))
        node.RWMutex.RLock()
        for addr := range node.Hashgraph {
            knownEventNums[addr] = len(node.Hashgraph[addr])
        }

        if verbose == 2 {
            fmt.Print("Known Events:\n")
            for addr, num := range knownEventNums {
                fmt.Printf("\t%s : %d\n", addr, num)
            }
        }

        // Ask the chosen peer how many events they do not know but I know
        peerRPCConnection, err := rpc.Dial("tcp", randomPeer)
        handleError(err)
        numEventsToSend := make(map[string]int, len(node.Hashgraph))
        _ = peerRPCConnection.Call("Node.GetNumberOfMissingEvents", knownEventNums, &numEventsToSend)

        if verbose == 2 {
            fmt.Print("Events to send:\n")
            for addr, num := range numEventsToSend {
                fmt.Printf("\t%s : %d\n", addr, num)
            }
        }

        // Send the missing events
        missingEvents := make(map[string][]*hashgraph.Event, len(numEventsToSend))
        for addr := range numEventsToSend {
            if numEventsToSend[addr] > 0 { /* it is possible for this to be negative, but that is ok, it just means the peer knows stuff I do not, which I will eventually learn via gossip */
                totalNumEvents := len(node.Hashgraph[addr])
                for _, event := range node.Hashgraph[addr][totalNumEvents-numEventsToSend[addr]:] {
                    missingEvents[addr] = append(missingEvents[addr], event)
                }
            }
        }

        // Wrap the missing events in a struct for rpc, attach my own address here
        syncEventsDTO := hashgraph.SyncEventsDTO{
            SenderAddress: node.Address,
            MissingEvents: missingEvents,
        }

        if verbose == 2 {
            fmt.Println("remotely calling SyncAllEvents")
        }

        _ = peerRPCConnection.Call("Node.SyncAllEvents", syncEventsDTO, nil) // todo: one peer gets stuck here
        _ = peerRPCConnection.Close()
        node.RWMutex.RUnlock()

        if verbose == 2 {
            fmt.Println("exiting remote call to SyncAllEvents")
        }

        time.Sleep(gossipWaitTime)

    }
}

// Serves RPC calls in a go routine
func listenForRPCConnections(listener *net.TCPListener) {
    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        rpc.ServeConn(conn)
    }
}

// Returns the local address of this device
func getLocalAddress() string {
    conn, err := net.Dial("udp", "eng.ku.edu.tr:80")
    handleError(err)
    defer func() {
        handleError(conn.Close())
    }()
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP.String()
}

// Auxiliary for any error
func handleError(e error) {
    if e != nil {
        panic(e)
    }
}
