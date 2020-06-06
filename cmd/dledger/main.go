package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"../../pkg/dledger"
)

const (
	defaultPort = "8080" // Use this default port if it isn't specified via command line arguments.
)

func main() {
	port := defaultPort
	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	distributedLedger := dledger.NewDLedger(port, "peers.txt")

	distributedLedger.WaitForPeers()
	fmt.Printf("I am online at %s and all peers are available.\n", distributedLedger.MyAddress)
	distributedLedger.Start()

	// Routine for user transaction inputs
	var input int
	var amount float64
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println()
	for {
		// note: peerAddressMap contains me, but peerAddresses does not
		fmt.Printf("\nDear %s, please choose a client for your new transaction.\n", distributedLedger.PeerAddressMap[distributedLedger.MyAddress])
		for i, addr := range distributedLedger.PeerAddresses {
			fmt.Printf("\t%d) %s\n", i+1, distributedLedger.PeerAddressMap[addr])
		}

		fmt.Printf("Enter a number: > ")
		errForInput := true
		for errForInput {
			var err error
			errForInput = false
			scanner.Scan()
			input, err = strconv.Atoi(scanner.Text())
			if err != nil || input <= 0 || input > len(distributedLedger.PeerAddresses) {
				errForInput = true
				fmt.Printf("\nBad input, try again: > ")
			}
		}
		chosenAddr := distributedLedger.PeerAddresses[input-1]

		fmt.Printf("\nDear %s, please enter how much credits would you like transfer to %s:\n\t> ",
			distributedLedger.PeerAddressMap[distributedLedger.MyAddress], distributedLedger.PeerAddressMap[chosenAddr])
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

		distributedLedger.PerformTransaction(chosenAddr, amount)
		fmt.Printf("\nSuccessfully added transaction:\n\t'%s sends %f to %s'\n", distributedLedger.PeerAddressMap[distributedLedger.MyAddress], amount, distributedLedger.PeerAddressMap[chosenAddr])
	}

}
