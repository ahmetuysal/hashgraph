package hashgraph

// todo: godoc documentations needed

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
)

const (
	verbose           = 0  // 1: full, 2: necessary prints. Use for debugging, default to 0
	signatureByteSize = 64 // Number of bytes for the signature
)

//Node : A member of the distributed ledger system. Is identified by it's address.
type Node struct {
	sync.RWMutex
	Address                       string                       // ip:port of the peer
	Hashgraph                     map[string][]*Event          // local copy of hashgraph, map to peer address -> peer events
	Events                        map[string]*Event            // events as a map of signature -> event
	Witnesses                     map[string]map[uint32]*Event // map of peer addres -> (map of round -> witness)
	FirstRoundOfFameUndecided     map[string]uint32            // the round of first witness that's fame is undecided for each peer
	FirstEventOfNotConsensusIndex map[string]int               // the index of first non-consensus event
	ConsensusEvents               []*Event                     // list of events with roundReceived and consensusTimestamp
	TransactionBuffer             []Transaction                // slice of transactions stored until next gossip
}

//Transaction : A statement of money transfer from a sender to a receiver.
type Transaction struct {
	SenderAddress   string  // ip:port of sender
	ReceiverAddress string  // ip:port of receiver
	Amount          float64 // amount
}

//SyncEventsDTO : Data Transfer Object for SyncAllEvents function
type SyncEventsDTO struct {
	SenderAddress string              // address of the node who made the call
	MissingEvents map[string][]*Event // map of addresses to events of those addresses that are missing on the remotely called node
}

//GetNumberOfMissingEvents : Node A calls Node B to learn which events B does not know and A knows.
func (n *Node) GetNumberOfMissingEvents(numEventsAlreadyKnown map[string]int, numEventsToSend *map[string]int) error {
	n.RWMutex.RLock()
	for addr := range n.Hashgraph {
		(*numEventsToSend)[addr] = numEventsAlreadyKnown[addr] - len(n.Hashgraph[addr])
	}
	n.RWMutex.RUnlock()
	return nil
}

//SyncAllEvents : Node A first calls GetNumberOfMissingEvents on B, and then sends the missing events in this function
func (n *Node) SyncAllEvents(events SyncEventsDTO, success *bool) error {
	n.RWMutex.Lock()

	if verbose == 1 {
		fmt.Printf("Syncing nodes %s and %s...\n", events.SenderAddress, n.Address)
		fmt.Printf("Hashgraph lengths BEFORE:\n[")
		for addr := range n.Hashgraph {
			fmt.Printf("%d ", len(n.Hashgraph[addr]))
		}
		fmt.Printf("]\n")
	}

	// Add the missing events to my local hashgraph
	for addr := range events.MissingEvents {
		for _, missingEvent := range events.MissingEvents[addr] {

			if verbose == 1 {
				fmt.Printf("Adding missing event %s\n", missingEvent.Signature)
			}

			n.Hashgraph[addr] = append(n.Hashgraph[addr], missingEvent)
			n.Events[missingEvent.Signature] = missingEvent
			if missingEvent.IsWitness {
				n.Witnesses[missingEvent.Owner][missingEvent.Round] = missingEvent
			}

		}
	}

	if verbose == 1 {
		for sig, event := range n.Events {
			fmt.Printf("Signature %s, Event %s\n", sig, event.Signature)
		}
		fmt.Printf("Hashgraph lengths AFTER:\n[")
		for addr := range n.Hashgraph {
			fmt.Printf("%d ", len(n.Hashgraph[addr]))
		}
		fmt.Printf("]\n")
	}

	// Store the transactions temporarily, and reset the global buffer
	transactions := n.TransactionBuffer
	n.TransactionBuffer = nil

	// Create random signature
	signatureUUID, err := uuid.NewV4()
	handleError(err)
	signature := signatureUUID.String()

	// Assign parents
	newEventsSelfParent := n.Hashgraph[n.Address][len(n.Hashgraph[n.Address])-1]
	newEventsOtherParent := n.Hashgraph[events.SenderAddress][len(n.Hashgraph[events.SenderAddress])-1]

	if verbose == 1 {
		fmt.Printf("Self round: %d\tOther round %d\n", newEventsSelfParent.Round, newEventsOtherParent.Round)
	}

	// Create event
	newEvent := Event{
		Owner:              n.Address,
		Signature:          signature, // todo: use RSA
		SelfParentHash:     newEventsSelfParent.Signature,
		OtherParentHash:    newEventsOtherParent.Signature,
		Timestamp:          time.Now(),
		Transactions:       transactions,
		Round:              0,
		IsWitness:          false,
		IsFamous:           false,
		IsFameDecided:      false,
		RoundReceived:      0,
		ConsensusTimestamp: time.Unix(0, 0),
	}

	if verbose == 1 {
		fmt.Println("entering DivideRounds")
	}

	// Find the round & witness of new event
	n.DivideRounds(&newEvent)

	if verbose == 1 {
		fmt.Println("exiting DivideRounds\nentering DecideFame")
	}

	// Update local arrays
	if newEvent.IsWitness {
		n.Witnesses[newEvent.Owner][newEvent.Round] = &newEvent
	}
	n.Events[newEvent.Signature] = &newEvent
	n.Hashgraph[n.Address] = append(n.Hashgraph[n.Address], &newEvent)

	// Decide fame on fame-undecided witnesses
	n.DecideFame()

	if verbose == 1 {
		fmt.Println("exiting DecideFame\nentering FindOrder")
	}

	// Arrive to consensus on order of events
	n.FindOrder()

	if verbose == 1 {
		fmt.Println("exiting FindOrder")
	}

	*success = true

	if verbose == 1 {
		fmt.Printf("Nodes %s and %s successfully synced.\n", events.SenderAddress, n.Address)
	}

	n.RWMutex.Unlock()
	return nil
}

//DivideRounds : Calculates the round of a new event
func (n *Node) DivideRounds(e *Event) {
	if verbose == 1 {
		fmt.Printf("New event: %v\n\n", *e)
	}

	selfParent, okSelfParent := n.Events[e.SelfParentHash]
	otherParent, okOtherParent := n.Events[e.OtherParentHash]
	if !okSelfParent || !okOtherParent {
		fmt.Printf("Parents were not ok: (self: %t, other: %t)\n", okSelfParent, okOtherParent)
		return
	}

	// Round of this event is at least the round of it's parents
	r := max(selfParent.Round, otherParent.Round)

	if verbose == 1 && r == 0 {
		fmt.Println(*selfParent, "\n", *otherParent)
	}
	if verbose == 1 {
		fmt.Printf("entering findWitnessesOfARound(%d)\n", r)
	}

	// Get round r witnesses
	witnesses := n.findWitnessesOfARound(r)
	if verbose == 1 {
		fmt.Printf("exited findWitnessesOfARound(%d)\nchecking strongly see for witnesses\n", r)
	}

	// Count strongly seen witnesses in round r
	stronglySeenWitnessCount := 0
	for _, w := range witnesses {
		if n.stronglySee(e, w) {
			stronglySeenWitnessCount++
		}
	}
	if verbose == 1 {
		fmt.Println("finished strongly see checks")
	}

	// Check supermajority
	if stronglySeenWitnessCount > int(math.Ceil(2.0*float64(len(n.Hashgraph))/3.0)) {
		e.Round = r + 1
	} else {
		e.Round = r
	}

	// Check if this new event is a witness
	if e.Round > selfParent.Round { // unlike the original algorithm, we do not check if there is no self parent as we never create the initial event here
		e.IsWitness = true
	}
}

//DecideFame : Decides if a witness is famous or not
func (n *Node) DecideFame() {
	// Get the last witnesses that does not have a decided fame
	var fameUndecidedWitnesses []*Event // this is "for each x" in the paper
	for addr := range n.Hashgraph {
		for round, witness := range n.Witnesses[addr] { // todo: optimize the access
			if round >= n.FirstRoundOfFameUndecided[addr] {
				fameUndecidedWitnesses = append(fameUndecidedWitnesses, witness)
			}
		}
	}

	for _, e := range fameUndecidedWitnesses {
		// Get all witnesses that have greater rounds
		var witnessesWithGreaterRounds []*Event
		for addr := range n.Hashgraph {
			for round, witness := range n.Witnesses[addr] { // todo: optimize the access
				if round > e.Round {
					witnessesWithGreaterRounds = append(witnessesWithGreaterRounds, witness)
				}
			}
		}

		for _, w := range witnessesWithGreaterRounds {
			// Find witnesses of prior round
			witnessesOfRound := n.findWitnessesOfARound(w.Round - 1)

			// Choose the strongly seen ones
			var stronglySeenWitnessesOfRound []*Event
			for _, wr := range witnessesOfRound {
				if n.stronglySee(w, wr) {
					stronglySeenWitnessesOfRound = append(stronglySeenWitnessesOfRound, wr)
				}
			}

			// Count votes
			votes := make([]bool, len(stronglySeenWitnessesOfRound))
			majority := 0
			trueVotes := 0
			falseVotes := 0
			for i, voter := range stronglySeenWitnessesOfRound {
				if n.see(voter, e) {
					votes[i] = true
					majority++
					trueVotes++
				} else {
					votes[i] = false
					majority--
					falseVotes++
				}
			}
			majorityVote := majority >= 0
			superMajorityThreshold := int(math.Ceil(2.0 * float64(len(n.Hashgraph)) / 3.0))
			if (majorityVote && trueVotes > superMajorityThreshold) || (!majorityVote && falseVotes > superMajorityThreshold) {
				e.IsFamous = majorityVote
				e.IsFameDecided = true
				n.FirstRoundOfFameUndecided[e.Owner] = e.Round + 1
				break
			}
		}
	}
}

//FindOrder : Arrive at a consensus on the order of events
func (n *Node) FindOrder() {
	// Find events
	var nonConsensusEvents []*Event
	for addr := range n.Hashgraph {
		nonConsensusEvents = append(nonConsensusEvents, n.Hashgraph[addr][n.FirstEventOfNotConsensusIndex[addr]:]...)
	}
	for _, e := range nonConsensusEvents {
		// First & Third conditions: find a valid round number
		r := n.FirstRoundOfFameUndecided[e.Owner] // owner is arbitrary
		for _, round := range n.FirstRoundOfFameUndecided {
			if round < r {
				r = round
			}
		}
		witnesses := n.findWitnessesOfARound(r)
		if len(witnesses) != 0 {
			// Second condition: make sure x is seen by all famous witnesses
			condMet := true
			for _, w := range witnesses {
				if w.IsFamous && !n.see(w, e) {
					condMet = false
					break
				}
			}
			if condMet {
				// Construct consensus set
				var s []*Event
				for _, w := range witnesses {
					z := w
					for !isInitial(z) {
						if z.Round < r {
							// if z is lower than e, e can't be ancestor of z
							break
						}
						if n.see(z, e) && !n.see(n.Events[z.SelfParentHash], e) {
							s = append(s, z)
						}
						z = n.Events[z.SelfParentHash]
					}
				}
				if len(s) != 0 {
					e.RoundReceived = r
					// Take median
					timestamps := make(timeSlice, len(s))
					for i, se := range s {
						timestamps[i] = se.Timestamp
					}
					if verbose == 1 {
						fmt.Printf("Lenghts | timestamps: %d\t s: %d\t witnesses: %d\n", len(timestamps), len(s), len(witnesses))
					}

					sort.Stable(timestamps) // returns timestamps sorted in increasing order
					medianTimestamp := timestamps[int(math.Floor(float64(len(timestamps))/2.0))]
					e.ConsensusTimestamp = medianTimestamp
					n.ConsensusEvents = append(n.ConsensusEvents, e)
				}
			}
		}
	}
	// Bring all consensus events ordered
	consensusSlice := make(eventPtrSlice, len(n.ConsensusEvents))
	consensusSlice = n.ConsensusEvents
	sort.Stable(consensusSlice)
	n.ConsensusEvents = consensusSlice
}

// If we can reach to target using downward edges only, we can see it. Downward in this case means that we reach through either parent. This function is used for voting
func (n *Node) see(current *Event, target *Event) bool {
	if current.Signature == target.Signature || (current.Round > target.Round && current.Owner == target.Owner) {
		return true
	}
	if (current.Round < target.Round) || (current.IsWitness && current.Round == target.Round) || isInitial(current) {
		return false
	}
	return n.see(n.Events[current.SelfParentHash], target) || n.see(n.Events[current.OtherParentHash], target)
}

// If we see the target, and we go through 2n/3 different nodes as we do that, we say we strongly see that target. This function is used for choosing the famous witness
func (n *Node) stronglySee(current *Event, target *Event) bool {
	if verbose == 1 {
		fmt.Println("entering getLatestAncestorFromAllNodes")
	}
	latestAncestors := n.getLatestAncestorFromAllNodes(current, target.Round)
	if verbose == 1 {
		fmt.Println("exited getLatestAncestorFromAllNodes")
	}
	count := 0
	for _, latestAncestor := range latestAncestors {
		if n.see(latestAncestor, target) {
			count++
		}
	}

	return count > int(math.Ceil(2.0*float64(len(n.Hashgraph))/3.0))
}

// Do breadth first search to find the latest ancestor that event e can see on every node
func (n *Node) getLatestAncestorFromAllNodes(e *Event, minRound uint32) map[string]*Event {
	latestAncestors := make(map[string]*Event, len(n.Hashgraph))
	if !isInitial(e) {
		// Queue for BFS
		var queue []*Event
		queue = append(queue, e)

		var currentEvent *Event
		for len(queue) > 0 {

			if verbose == 1 {
				fmt.Printf("Q: %v\n\n", queue)
			}

			// Pop queue
			currentEvent = queue[0]
			queue[0] = nil
			queue = queue[1:]

			// Check if we have assigned an ancestor for this address yet
			currentAncestorFromOwner, ok := latestAncestors[currentEvent.Owner]

			if !ok {
				// if not, assign current
				latestAncestors[currentEvent.Owner] = currentEvent
			} else if (currentEvent.Round >= currentAncestorFromOwner.Round) && (currentEvent.Owner == currentAncestorFromOwner.Owner || n.see(currentEvent, currentAncestorFromOwner)) {
				// if we did, check if current event is higher up
				latestAncestors[currentEvent.Owner] = currentEvent
			}

			// If current event is initial, it has no parents, so we should not bother with next iteration
			if !isInitial(currentEvent) {
				if verbose == 1 && n.Events[currentEvent.SelfParentHash] == currentEvent {
					fmt.Printf("My parent is myself %+v\n", currentEvent)
				}

				// Add self parent to the queue if round is still higher
				selfParent, ok := n.Events[currentEvent.SelfParentHash]
				if ok && selfParent.Round >= minRound {
					queue = append(queue, selfParent)
				}

				// Add other parent to the queue if round is still higher
				otherParent, ok := n.Events[currentEvent.OtherParentHash]
				if ok && otherParent.Round >= minRound {
					queue = append(queue, otherParent)
				}
			}
		}
	}
	return latestAncestors
}

// Find witnesses of round r, which is the first event with round r in every node
// note that it is possible that a node does not have a witness on a round r while the others do
func (n *Node) findWitnessesOfARound(r uint32) map[string]*Event {
	witnesses := make(map[string]*Event, len(n.Hashgraph))
	for addr := range n.Hashgraph {
		w, ok := n.Witnesses[addr][r]
		if ok {
			witnesses[addr] = w
		}
	}
	return witnesses
}

// There is no built-in max function for uint32...
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func isInitial(e *Event) bool {
	return e.SelfParentHash == "" || e.OtherParentHash == ""
}

/** timeSlice interface for sorting **/
type timeSlice []time.Time

func (p timeSlice) Len() int {
	return len(p)
}
func (p timeSlice) Less(i, j int) bool {
	return p[i].Before(p[j])
}
func (p timeSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

/** eventSlice interface for sorting **/
type eventPtrSlice []*Event

func (p eventPtrSlice) Len() int {
	return len(p)
}
func (p eventPtrSlice) Less(i, j int) bool {
	if p[i].RoundReceived == p[j].RoundReceived {
		// round recieved may be same, break ties with timestamp
		if p[i].ConsensusTimestamp == p[j].ConsensusTimestamp {
			// timestamp may be same (though very unlikely)
			return true
		}
		return p[i].ConsensusTimestamp.Before(p[j].ConsensusTimestamp)
	}
	return p[i].RoundReceived < p[j].RoundReceived
}
func (p eventPtrSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func handleError(e error) {
	if e != nil {
		panic(e)
	}
}
