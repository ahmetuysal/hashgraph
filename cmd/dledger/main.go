package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"time"

	"../../pkg/hashgraph"
)

const (
	verbose                    = true                    // Set true for debug printing
	defaultPort                = "8080"                  // Use this default port if it isn't specified via command line arguments.
	gossipWaitTime             = 1000 * time.Millisecond // the amount of time.sleep milliseconds between each random gossip
	connectionAttemptDelayTime = 100 * time.Millisecond  // the amount of time.sleep milliseconds between each connection attempt
)

func main() {
	port := defaultPort
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	localIPAddress := getLocalAddress()
	myAddress := localIPAddress + ":" + port
	peerAddressMap := readPeerAddresses("peers.txt", localIPAddress) // a map with ip:port -> name (Alice, Bob, etc.) for clarity

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
	initialHashgraph := make(map[string][]*hashgraph.Event, len(peerAddressMap))
	for addr := range peerAddressMap {
		initialHashgraph[addr] = make([]*hashgraph.Event, 0) // We should not know any event other than our own event at the start
	}
	initialEvent := hashgraph.Event{
		Owner:           myAddress,
		Signature:       time.Now().String(), // todo: use RSA
		SelfParentHash:  "",
		OtherParentHash: "",
		Timestamp:       time.Now(),
		Transactions:    nil,
		Round:           1,
		IsWitness:       true, // true because the initial event is the first event of its round
		IsFamous:        false,
	}
	initialHashgraph[myAddress] = append(initialHashgraph[myAddress], &initialEvent)
	myNode := hashgraph.Node{
		Address:                       myAddress,
		Hashgraph:                     initialHashgraph,
		Events:                        make(map[string]*hashgraph.Event),
		Witnesses:                     make(map[string]map[uint32]*hashgraph.Event),
		FirstRoundOfFameUndecided:     make(map[string]uint32),
		FirstEventOfNotConsensusIndex: make(map[string]int),
	}
	myNode.Witnesses[initialEvent.Owner] = make(map[uint32]*hashgraph.Event)
	myNode.Witnesses[initialEvent.Owner][1] = &initialEvent
	myNode.Events[initialEvent.Signature] = &initialEvent
	myNode.FirstRoundOfFameUndecided[initialEvent.Owner] = 1
	myNode.FirstEventOfNotConsensusIndex[initialEvent.Owner] = 0 // index 0 for the initial event

	// Setup the server
	_ = rpc.Register(&myNode)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", myAddress)
	listener, _ := net.ListenTCP("tcp", tcpAddr)
	go listenForRPCConnections(listener)

	// Assert that everyone is online
	checkPeersForStart(peerAddresses)
	fmt.Printf("I am online at %s and all peers are available.\n", myAddress)

	go gossipRoutine(myNode, peerAddresses)

	// Routine for user transaction inputs
	var input int
	var amount float64
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println()
	for {
		// note: peerAddressMap contains me, but peerAddresses does not
		fmt.Printf("\nDear %s, please choose a client for your new transaction.\n", peerAddressMap[myAddress])
		for i, addr := range peerAddresses {
			fmt.Printf("\t%d) %s\n", i+1, peerAddressMap[addr])
		}

		fmt.Printf("Enter a number: > ")
		errForInput := true
		for errForInput {
			var err error
			errForInput = false
			scanner.Scan()
			input, err = strconv.Atoi(scanner.Text())
			if err != nil || input <= 0 || input > len(peerAddresses) {
				errForInput = true
				fmt.Printf("\nBad input, try again: > ")
			}
		}
		chosenAddr := peerAddresses[input-1]

		fmt.Printf("\nDear %s, please enter how much credits would you like transfer to %s:\n\t> ", peerAddressMap[myAddress], peerAddressMap[chosenAddr])
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

		myNode.TransactionBuffer = append(myNode.TransactionBuffer, hashgraph.Transaction{SenderAddress: myAddress, ReceiverAddress: chosenAddr, Amount: amount})
		fmt.Printf("\nSuccessfully added transaction:\n\t'%s sends %f to %s'\n", peerAddressMap[myAddress], amount, peerAddressMap[chosenAddr])
	}

}

func gossipRoutine(node hashgraph.Node, peerAddresses []string) {
	for {
		randomPeer := peerAddresses[rand.Intn(len(peerAddresses))]

		knownEventNums := make(map[string]int, len(node.Hashgraph))
		for addr := range node.Hashgraph {
			knownEventNums[addr] = len(node.Hashgraph[addr])
		}

		if verbose {
			fmt.Print("Known Events: ")
			fmt.Println(knownEventNums)
		}

		peerRPCConnection, err := rpc.Dial("tcp", randomPeer)
		handleError(err)
		numEventsToSend := make(map[string]int, len(node.Hashgraph))
		_ = peerRPCConnection.Call("Node.GetNumberOfMissingEvents", knownEventNums, &numEventsToSend)

		if verbose {
			fmt.Print("Events to send: ")
			fmt.Println(numEventsToSend)
		}

		missingEvents := make(map[string][]hashgraph.Event, len(numEventsToSend))
		for addr := range numEventsToSend {
			if numEventsToSend[addr] > 0 {
				totalNumEvents := len(node.Hashgraph[addr])

				for _, event := range node.Hashgraph[addr][totalNumEvents-numEventsToSend[addr]:] {
					missingEvents[addr] = append(missingEvents[addr], *event)
				}
			}
		}

		syncEventsDTO := hashgraph.SyncEventsDTO{
			SenderAddress: node.Address,
			MissingEvents: missingEvents,
		}

		if verbose {
			fmt.Println("moving on 1")
		}
		_ = peerRPCConnection.Call("Node.SyncAllEvents", syncEventsDTO, nil)
		_ = peerRPCConnection.Close()

		if verbose {
			fmt.Println("moving on 2")
		}
		time.Sleep(gossipWaitTime)

	}
}

func readPeerAddresses(path string, localIPAddr string) map[string]string {
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
		peers[strings.Replace(addrName[0], "localhost", localIPAddr, 1)] = addrName[1]
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

func checkPeersForStart(peerAddresses []string) {
	fmt.Println("Welcome to Decentralized Ledger, please wait until all other peers are available.")
	peerAvailable := make([]bool, len(peerAddresses))
	remainingPeers := len(peerAddresses)
	for remainingPeers > 0 {
		for index, isAlreadyResponded := range peerAvailable {
			// we have already reached this peer
			if isAlreadyResponded {
				continue
			}

			rpcConnection, err := rpc.Dial("tcp", peerAddresses[index])
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
