package main

import (
    "bufio"
    "fmt"
    "net"
    "net/rpc"
    "os"
    "strconv"
    "strings"
    "time"
)

////////////////////////////// 0: GLOBALS ////////////////////////////////////////
var myPeerNo int          // the position of my ip:port in the peers.txt, given as a cmdline arg
var myIP string           // my IP, read from peers.txt
var myPort string         // my port, read from peers.txt
var myName string         // my name, such as Alice, Bob, etc.
var peers map[string]Peer // dictionary with key as ip:port and value as Peer struct

//Transaction : A transaction in an Event of Hashgraph
type Transaction struct {
    senderAddress   string  // ip:port of sender
    recieverAddress string  // ip:port of receiver
    amount          float64 // amount
}

//Event : An Event is a point in the Hashgraph
type Event struct {
    signature       string         // Event should be signed by it's creator
    selfParentHash  string         // Hash of the self-parent, which is the hash for the event before this event in my timeline.
    otherParentHash string         // Hash of the other-parent, which is the hash for the last event of the peer that called me.
    timestamp       int            // Should we use vector clocks for this? Or, what to the authors use?
    transactions    *[]Transaction // List of transactions for this event, size can be 0 too.
}

//Hashgraph : Each peer holds a copy of the Hashgraph in their memory.
type Hashgraph struct {
    events *[]Event // Hashgraph holds the list of events
}

//Peer : A Peer has a name, a number from my perspective, an address and a responding Client object for me to communicate with
type Peer struct {
    address string      // ip:port of the peer
    name    string      // name of the peer (Alice, Bob, etc.)
    pClient *rpc.Client // pointer to the Client object
    no      int         // number of the peer from my perspective
}

////////////////////////////// 1: MAIN ///////////////////////////////////////////
func main() {

    // We expect a number in the command line for the peer to know it's own ID as it reads from peers.txt
    if len(os.Args) != 2 {
        fmt.Println("Usage: ", os.Args[0], "<peer no (min 1)>")
        os.Exit(1)
    }
    myPeerNo, _ = strconv.Atoi(os.Args[1])

    // Read the peer ip and ports from the file
    var peersToConnect []string
    ipandports, err := readLines("peers.txt")
    checkError(err)
    for i, line := range ipandports {
        if i == myPeerNo-1 {
            myName = strings.Split(line, " ")[1]
            myIP = strings.Split(strings.Split(line, " ")[0], ":")[0]
            myPort = strings.Split(strings.Split(line, " ")[0], ":")[1]
        } else {
            peersToConnect = append(peersToConnect, line)
        }
    }

    // Start the server
    tcpAddr, err := net.ResolveTCPAddr("tcp", ":"+myPort)
    checkError(err)
    listener, err := net.ListenTCP("tcp", tcpAddr)
    checkError(err)
    fmt.Printf("> Peer %d started successfully.\n", myPeerNo)

    // Branch into the communicator routine
    go communicatorRoutine(peersToConnect)
    // perhaps we need another routine for random gossips? // TODO Gossip routine

    // Serve RPC connections
    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        go rpc.ServeConn(conn)
    }
}

////////////////////////////// 2: COMMUNICATOR ///////////////////////////////////
func communicatorRoutine(peersToConnect []string) {
    // Connect to peer in the peers.txt
    connectToPeers(peersToConnect)
    fmt.Printf("All %d peers are connected here!\n\n", len(peers)+1) // +1 including me :)

    // Start the infinite loop for user interface
    var input int
    var amount float64
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Printf("\nDear %s, please choose a client for your transaction.\n", myName)
        for _, peerObject := range peers {
            fmt.Printf("\t%d) %s\n", peerObject.no, peerObject.name)
        }
        fmt.Printf("Enter a number: > ")
        errForInput := true
        for errForInput {
            var err error
            errForInput = false
            scanner.Scan()
            input, err = strconv.Atoi(scanner.Text())
            if err != nil || input <= 0 || input > len(peers) {
                errForInput = true
                fmt.Printf("\nBad input, try again: > ")
            }
        }
        // User made a choice
        var chosenPeer *Peer
        for _, peerObject := range peers {
            input--
            if input == 0 {
                // We choose this peer
                chosenPeer = &peerObject
            }
        }
        fmt.Printf("\nDear %s, please enter how much credits would you like transfer to %s:\n\t> ", myName, chosenPeer.name)
        errForInput = true
        for errForInput {
            var err error
            errForInput = false
            scanner.Scan()
            amount, err = strconv.ParseFloat(scanner.Text(), 64)
            if err != nil || amount <= 0 {
                errForInput = true
                fmt.Printf("Bad input, try again: > ")
            }
        }
        // User chose an amount
        fmt.Printf("\nCommitting transaction:\n\t%s sends %f to %s\n", myName, amount, chosenPeer.name)
        // TODO TODO TODO HASHGRAPH TRANSACTION EVENTS AND STUFF
    }
}

////////////////////////////// 3: AUXILLARIES ////////////////////////////////////
// Error checking auxillary function
func checkError(err error) {
    if err != nil {
        fmt.Println("Fatal error ", err.Error())
        os.Exit(1)
    }
}

// File read auxillary function
func readLines(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return lines, scanner.Err()
}

// Graceful termination (this function, if initiated from any peer, will tell every peer to terminate, and then finally terminate itself too)
/*
func terminateAll() {

	fmt.Printf("Terminating in a second...\n")
	time.Sleep(1000 * time.Millisecond)
	os.Exit(0)
}
*/

// Connects to the list of addresses, returns their Client object pointers, numbers and names.
func connectToPeers(peersToConnect []string) {
    // Attempt to connect to all the peers in a cyclic fashion
    time.Sleep(500 * time.Millisecond)
    peers = make(map[string]Peer) // Initialize the map
    i := 0
    for {
        client, err := rpc.Dial("tcp", strings.Split(peersToConnect[i], " ")[0]) // connecting to the service
        if err != nil {
            // Error occured but it is ok, maybe other servers are setting up at the moment
            fmt.Printf("Dialed %s but no response...\n", strings.Split(peersToConnect[i], " ")[0])
            // Delay just a bit
            time.Sleep(500 * time.Millisecond)
        } else {
            // Connection established
            fmt.Printf("Connected to %s\n", strings.Split(peersToConnect[i], " ")[0])
            peers[peersToConnect[i]] = Peer{address: strings.Split(peersToConnect[i], " ")[0], name: strings.Split(peersToConnect[i], " ")[1], no: i + 1, pClient: client} // add struct to the dictionary
            peersToConnect = append(peersToConnect[:i], peersToConnect[i+1:]...)                                                                                           // update the list of remaining peers to connect
        }
        if len(peersToConnect) != 0 {
            i = (i + 1) % len(peersToConnect)
        } else {
            break
        }
    }
    // We do not return anything here because the peers dictionary is global and we update it
}


func hashgraphMain() {
    for {
        // TODO: select a node at random
        // TODO: sync all events with that node
        // TODO: create a new event
        // divideRounds()
        // decideFame()
        // findOrder()
        fmt.Println("Hello")
    }
}