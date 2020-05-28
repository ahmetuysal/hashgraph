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
var myPeerNo int   // the position of my ip:port in the peers.txt, given as a cmdline arg
var myIP string    // my IP, read from peers.txt
var myPort string  // my port, read from peers.txt
var peers []string // holds the list of ip and ports from peers.txt except my own

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
			myIP = strings.Split(line, ":")[0]
			myPort = strings.Split(line, ":")[1]
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

	// Serve RPC connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go rpc.ServeConn(conn) // it is important for this to be a goroutine
	}
}

////////////////////////////// 2: COMMUNICATOR ///////////////////////////////////
func communicatorRoutine(peersToConnect []string) {
	peerClients := connectToPeers(peersToConnect)
	/// TODO TODO TODO ///
	fmt.Printf("All %d peers are connected here!\n\n", len(peerClients)+1)

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

func connectToPeers(peersToConnect []string) []*rpc.Client {
	// Attempt to connect to all the peers in a cyclic fashion
	time.Sleep(500 * time.Millisecond)
	var peerClients []*rpc.Client
	i := 0
	for {
		client, err := rpc.Dial("tcp", peersToConnect[i]) // connecting to the service
		if err != nil {
			// Error occured but it is ok, maybe other servers are setting up at the moment
			fmt.Printf("Dialed %s but no response...\n", peersToConnect[i])
			// Delay just a bit
			time.Sleep(500 * time.Millisecond)
		} else {
			// Connection established
			fmt.Printf("Connected to %s\n", peersToConnect[i])
			peersToConnect = append(peersToConnect[:i], peersToConnect[i+1:]...)
			peerClients = append(peerClients, client)
		}

		if len(peersToConnect) != 0 {
			i = (i + 1) % len(peersToConnect)
		} else {
			break
		}

	}
	return peerClients
}
