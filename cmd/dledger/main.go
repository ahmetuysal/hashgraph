package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"

	"../../pkg/hashgraph"
)

const (
	DefaultPort = "8080" // Use this default port if it isn't specified via command line arguments
	VERBOSE     = true   // For debugging, we can set it to false or remove these alltogether after we are done
)

//Transaction : ...
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
	peerAddressMap := readPeerAddresses("peers.txt", localIPAddress) // a map with ip:port -> name (Alice, Bob, etc.) for clarity

	// Assert that your own address is on the peers file
	_, ok := peerAddressMap[myAddress]
	if !ok {
		panic("Peers file does not include my address: " + myAddress)
	}

	// Copy peer addresses to a slice for random access during gossip
	peerAddresses := make([]string, len(peerAddressMap))
	i := 0
	for k := range peerAddressMap {
		peerAddresses[i] = k // suggestion #1: do not add my own ip:port here, instead of creating another slice without me later
		i++
	}

	// Setup the Hashgraph
	initialHashgraph := make(map[string][]*hashgraph.Event, len(peerAddressMap))
	for addr := range peerAddressMap {
		initialHashgraph[addr] = make([]*hashgraph.Event, 0) // We should not know any event other than our own event at the start
	}
	initialEvent := hashgraph.Event{
		Owner:           myAddress,
		Signature:       time.Now().String(), // todo: use RSA
		SelfParentHash:  "",                  // suggestion #2: if selfParentHash == otherParentHash then an event is initial?
		OtherParentHash: "",
		Timestamp:       0, // todo: use datetime, perhaps 0 does not matter here
		Transactions:    nil,
		Round:           1,    // this is defined in the paper to be 1 for initial events
		IsWitness:       true, // true because this is the first event in this round
		IsFamous:        false,
	}
	initialHashgraph[myAddress] = append(initialHashgraph[myAddress], &initialEvent)
	myNode := hashgraph.Node{
		Address:   myAddress,
		Hashgraph: initialHashgraph,
		Events:    make(map[string]*hashgraph.Event),
	}
	myNode.Witnesses[initialEvent.Owner][1] = &initialEvent
	myNode.Events[initialEvent.Signature] = &initialEvent
	myNode.FirstRoundOfFameUndecided[initialEvent.Owner] = 1
	// Setup the server
	_ = rpc.Register(&myNode)
	tcpAddr, _ := net.ResolveTCPAddr("tcp", myAddress)
	listener, _ := net.ListenTCP("tcp", tcpAddr)
	go listenForRPCConnections(listener)

	// Assert that everyone is online
	checkPeersForStart(peerAddresses, myAddress)

	go hashgraphMain(myNode, peerAddresses)

	// todo: user interface for transactions
	for {
		continue
	}

}

func hashgraphMain(node hashgraph.Node, peerAddresses []string) {
	// see suggestion #1
	otherPeers := make([]string, len(peerAddresses)-1)
	i := 0
	for _, addr := range peerAddresses {
		if addr != node.Address {
			otherPeers[i] = addr
			i++
		}
	}

	// Gossip Loop
	for {
		randomPeer := otherPeers[rand.Intn(len(otherPeers))]

		knownEventNums := make(map[string]int, len(node.Hashgraph))
		for addr := range node.Hashgraph {
			knownEventNums[addr] = len(node.Hashgraph[addr])
		}

		if VERBOSE {
			fmt.Print("Known Events: ")
			fmt.Println(knownEventNums)
		}

		peerRPCConnection, err := rpc.Dial("tcp", randomPeer)
		handleError(err)
		numEventsToSend := make(map[string]int, len(node.Hashgraph))
		_ = peerRPCConnection.Call("Node.GetNumberOfMissingEvents", knownEventNums, &numEventsToSend)

		if VERBOSE {
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

		_ = peerRPCConnection.Call("Node.SyncAllEvents", syncEventsDTO, nil)
		_ = peerRPCConnection.Close()

		time.Sleep(5 * time.Second)

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
