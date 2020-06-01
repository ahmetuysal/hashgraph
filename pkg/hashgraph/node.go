package hashgraph

import (
	"fmt"
	"math"
	"sort"
	"time"
)

//Node :
type Node struct {
	Address                       string                       // ip:port of the peer
	Hashgraph                     map[string][]*Event          // local copy of hashgraph, map to peer address -> peer events
	Events                        map[string]*Event            // events as a map of signature -> event
	Witnesses                     map[string]map[uint32]*Event // map of peer addres -> (map of round -> witness)
	FirstRoundOfFameUndecided     map[string]uint32            // the round of first witness that's fame is undecided for each peer
	FirstEventOfNotConsensusIndex map[string]int               // the index of first non-consensus event
	ConsensusEvents               []*Event                     // list of events with roundReceived and consensusTimestamp
	TransactionBuffer             []Transaction
}

//Transaction : ...
type Transaction struct {
	SenderAddress   string  // ip:port of sender
	ReceiverAddress string  // ip:port of receiver
	Amount          float64 // amount
}

//SyncEventsDTO : Data transfer object for 2nd call in Gossip: SyncAllEvents
type SyncEventsDTO struct {
	SenderAddress string
	MissingEvents map[string][]Event // all missing events, map to peer address -> all events I don't know about this peer
}

//GetNumberOfMissingEvents : Node A calls Node B to learn which events B does not know and A knows.
func (n *Node) GetNumberOfMissingEvents(numEventsAlreadyKnown map[string]int, numEventsToSend *map[string]int) error {
	for addr := range n.Hashgraph {
		(*numEventsToSend)[addr] = numEventsAlreadyKnown[addr] - len(n.Hashgraph[addr])
	}
	return nil
}

//SyncAllEvents : Node A first calls GetNumberOfMissingEvents on B, and then sends the missing events in this function
func (n *Node) SyncAllEvents(events SyncEventsDTO, success *bool) error {
	for addr := range events.MissingEvents {
		for _, missingEvent := range events.MissingEvents[addr] {
			n.Hashgraph[addr] = append(n.Hashgraph[addr], &missingEvent)
			n.Events[missingEvent.Signature] = &missingEvent
		}
	}

	// Flush the transactions
	transactions := n.TransactionBuffer
	n.TransactionBuffer = nil
	newEvent := Event{
		Owner:           n.Address,
		Signature:       time.Now().String(), // todo: use RSA
		SelfParentHash:  n.Hashgraph[n.Address][len(n.Hashgraph[n.Address])-1].Signature,
		OtherParentHash: n.Hashgraph[events.SenderAddress][len(n.Hashgraph[events.SenderAddress])-1].Signature,
		Timestamp:       time.Now(),
		Transactions:    transactions, // todo: use the transaction buffer which grows with user input
		Round:           0,
		RoundReceived:   0,
		IsWitness:       false,
		IsFamous:        false,
	}
	n.Events[newEvent.Signature] = &newEvent
	n.Hashgraph[n.Address] = append(n.Hashgraph[n.Address], &newEvent)

	n.DivideRounds(&newEvent)
	n.DecideFame()
	n.FindOrder()

	return nil
}

//DivideRounds : Calculates the round of a new event
func (n Node) DivideRounds(e *Event) {
	selfParent := n.Events[e.SelfParentHash]
	otherParent := n.Events[e.OtherParentHash]
	r := max(selfParent.Round, otherParent.Round)
	witnesses := n.findWitnessesOfARound(r)
	stronglySeenWitnessCount := 0
	for _, w := range witnesses {
		if n.stronglySee(*e, *w) {
			stronglySeenWitnessCount++
		}
	}
	if stronglySeenWitnessCount > int(math.Ceil(2.0*float64(len(n.Hashgraph))/3.0)) {
		e.Round = r + 1
	} else {
		e.Round = r
	}
	if e.Round > selfParent.Round { // we do not check if there is no self parent, because we never create the initial event here
		e.IsWitness = true
		_, ok := n.Witnesses[e.Owner]
		if !ok {
			n.Witnesses[e.Owner] = make(map[uint32]*Event)
		}
		n.Witnesses[e.Owner][r] = e
	}

}

//DecideFame : Decides if a witness is famous or not
// note: we did not implement a coin round yet
func (n Node) DecideFame() {
	var fameUndecidedWitnesses []*Event // this is "for each x" in the paper
	for addr := range n.Hashgraph {
		for round, witness := range n.Witnesses[addr] { // todo: optimize the access
			if round >= n.FirstRoundOfFameUndecided[addr] {
				fameUndecidedWitnesses = append(fameUndecidedWitnesses, witness)
			}
		}
	}
	for _, e := range fameUndecidedWitnesses {
		var witnessesWithGreaterRounds []*Event
		for addr := range n.Hashgraph {
			for round, witness := range n.Witnesses[addr] { // todo: optimize the access
				if round > e.Round {
					witnessesWithGreaterRounds = append(witnessesWithGreaterRounds, witness)
				}
			}
		}
		// Decision starts now
		for _, w := range witnessesWithGreaterRounds {
			witnessesOfRound := n.findWitnessesOfARound(w.Round - 1)
			var stronglySeenWitnessesOfRound []*Event
			for _, wr := range witnessesOfRound {
				if n.stronglySee(*w, *wr) {
					stronglySeenWitnessesOfRound = append(stronglySeenWitnessesOfRound, wr)
				}
			}
			// Find majority vote
			votes := make([]bool, len(stronglySeenWitnessesOfRound))
			majority := 0
			trueVotes := 0
			falseVotes := 0
			for i, voter := range stronglySeenWitnessesOfRound {
				if n.see(*voter, *e) {
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
				n.FirstRoundOfFameUndecided[e.Owner] = e.Round + 1
				break
			}
		}
	}
}

//FindOrder : Arrive at a consensus on the order of events
func (n Node) FindOrder() {
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
				if w.IsFamous && !n.see(*w, *e) {
					condMet = false
					break
				}
			}
			if condMet {
				// Construct consensus set
				var s []*Event
				for _, w := range witnesses {
					z := w
					for !isInitial(*z) {
						if z.Round < r {
							// if z is lower than e, e can't be ancestor of z
							break
						}
						if n.see(*z, *e) && !n.see(*n.Events[z.SelfParentHash], *e) {
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
					fmt.Printf("Lenghts | timestamps: %d\t s: %d\t witnesses: %d\n", len(timestamps), len(s), len(witnesses))
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
func (n Node) see(current Event, target Event) bool {
	if current.Signature == target.Signature {
		return true
	}
	if (current.Round < target.Round) || isInitial(current) {
		return false
	}

	// Go has short-circuit evaluation, which we utilize here
	return n.see(*n.Events[current.SelfParentHash], target) || n.see(*n.Events[current.OtherParentHash], target)
}

// If we see the target, and we go through 2n/3 different nodes as we do that, we say we strongly see that target. This function is used for choosing the famous witness
func (n Node) stronglySee(current Event, target Event) bool {
	latestAncestors := n.getLatestAncestorFromAllNodes(current, target.Round)
	count := 0
	for _, latestAncestor := range latestAncestors {
		if n.see(*latestAncestor, target) {
			count++
		}
	}

	return count > int(math.Ceil(2.0*float64(len(n.Hashgraph))/3.0))
}

// todo: comment
func (n Node) getLatestAncestorFromAllNodes(e Event, minRound uint32) map[string]*Event {
	latestAncestors := make(map[string]*Event, len(n.Hashgraph))
	if !isInitial(e) {
		var queue []*Event
		queue = append(queue, &e)

		var currentEvent *Event
		for len(queue) > 0 {
			currentEvent = queue[0]
			queue[0] = nil
			queue = queue[1:]

			currentAncestorFromOwner, ok := latestAncestors[currentEvent.Owner]

			if !ok {
				latestAncestors[currentEvent.Owner] = currentEvent
			} else if currentEvent.Round >= currentAncestorFromOwner.Round && n.see(*currentEvent, *currentAncestorFromOwner) {
				latestAncestors[currentEvent.Owner] = currentEvent
			}

			if !isInitial(*currentEvent) {
				selfParent := n.Events[currentEvent.SelfParentHash]
				if selfParent.Round >= minRound {
					queue = append(queue, selfParent)
				}
				otherParent := n.Events[currentEvent.OtherParentHash]
				if otherParent.Round >= minRound {
					queue = append(queue, otherParent)
				}
			}

		}
	}
	return latestAncestors
}

// Find witnesses of round r, which is the first event with round r in every node
func (n Node) findWitnessesOfARound(r uint32) map[string]*Event {
	witnesses := make(map[string]*Event, len(n.Hashgraph))
	for addr := range n.Hashgraph {
		w, ok := n.Witnesses[addr][r]
		if ok {
			witnesses[addr] = w
		}
	}
	return witnesses // it is possible that a round does not have a witness on each node sometimes
}

// There is no built-in max function for uint32...
func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func isInitial(e Event) bool {
	return e.Round == 1 && e.IsWitness
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
			// timestamp may be same (though very unlikely), break by signature
			return true // TODO: use bits of RSA signature
		}
		return p[i].ConsensusTimestamp.Before(p[j].ConsensusTimestamp)
	}
	return p[i].RoundReceived < p[j].RoundReceived
}
func (p eventPtrSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
