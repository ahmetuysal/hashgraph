package hashgraph

import (
    "fmt"
    "math"
    "math/rand"
    "sort"
    "sync"
    "time"

    uuid "github.com/satori/go.uuid"
)

const (
    randomTransactionCount     = 2   // How many transactions to generate for each event
    randomTransactionAmountMax = 500 // Maximum amount in a random transaction
    randomTransactionAmountMin = 10  // Minimum amount in a random transaction
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
    seeDPMemory                   map[string]map[string]bool   // a map from p.Signature to q.Signature that yields whether it p sees q or not
}

//NewNode : Construct a new node for the distributed ledger
func NewNode(initialHashgraph map[string][]*Event, address string) *Node {
    return &Node{
        Address:                       address,
        Hashgraph:                     initialHashgraph,
        Events:                        make(map[string]*Event),
        Witnesses:                     make(map[string]map[uint32]*Event),
        FirstRoundOfFameUndecided:     make(map[string]uint32),
        FirstEventOfNotConsensusIndex: make(map[string]int),
        seeDPMemory:                   make(map[string]map[string]bool),
    }
}

//Transaction : A statement of money transfer from a sender to a receiver.
type Transaction struct {
    SenderAddress   string  `json:"sender_address"`   // ip:port of sender
    ReceiverAddress string  `json:"receiver_address"` // ip:port of receiver
    Amount          float64 `json:"amount"`           // amount
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
    otherPeerAddresses := make([]string, len(n.Hashgraph)-1)
    for addr := range n.Hashgraph {
        if addr != n.Address {
            otherPeerAddresses = append(otherPeerAddresses, addr)
        }

    }
    transactions := n.GenerateTransactions(randomTransactionCount, randomTransactionAmountMax, randomTransactionAmountMin, otherPeerAddresses)

    // Add the missing events to my local hashgraph
    for addr := range events.MissingEvents {
        for _, missingEvent := range events.MissingEvents[addr] {
            _, ok := n.Events[missingEvent.Signature]
            if !ok {
                n.Hashgraph[addr] = append(n.Hashgraph[addr], missingEvent)
                n.Events[missingEvent.Signature] = missingEvent
                if missingEvent.IsWitness {
                    n.Witnesses[missingEvent.Owner][missingEvent.Round] = missingEvent
                }
            }
        }
    }

    // Store the transactions temporarily, and reset the global buffer
    transactions = append(transactions, n.TransactionBuffer...)
    n.TransactionBuffer = nil

    // Create random signature
    signatureUUID, err := uuid.NewV4()
    handleError(err)
    signature := signatureUUID.String()

    // Assign parents
    newEventsSelfParent := n.Hashgraph[n.Address][len(n.Hashgraph[n.Address])-1]
    newEventsOtherParent := n.Hashgraph[events.SenderAddress][len(n.Hashgraph[events.SenderAddress])-1]

    // Create event
    newEvent := Event{
        Owner:              n.Address,
        Signature:          signature,
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

    // Find the round & witness of new event
    n.DivideRounds(&newEvent)

    // Update local arrays
    if newEvent.IsWitness {
        n.Witnesses[newEvent.Owner][newEvent.Round] = &newEvent
    }
    n.Events[newEvent.Signature] = &newEvent
    n.Hashgraph[n.Address] = append(n.Hashgraph[n.Address], &newEvent)

    // Decide fame on fame-undecided witnesses
    n.DecideFame()
    // Arrive to consensus on order of events
    n.FindOrder()

    *success = true
    n.RWMutex.Unlock()
    return nil
}

//DivideRounds : Calculates the round of a new event
func (n *Node) DivideRounds(e *Event) {
    selfParent, okSelfParent := n.Events[e.SelfParentHash]
    otherParent, okOtherParent := n.Events[e.OtherParentHash]
    if !okSelfParent || !okOtherParent {
        fmt.Printf("Parents were not ok: (self: %t, other: %t)\n", okSelfParent, okOtherParent)
        return
    }

    // Round of this event is at least the round of it's parents
    r := max(selfParent.Round, otherParent.Round)

    // Get round r witnesses
    witnesses := n.findWitnessesOfARound(r)

    // Count strongly seen witnesses in round r
    stronglySeenWitnessCount := 0
    for _, w := range witnesses {
        if n.stronglySee(e, w) {
            stronglySeenWitnessCount++
        }
    }

    // Check supermajority
    if float64(stronglySeenWitnessCount) > (2.0 * float64(len(n.Hashgraph)) / 3.0) {
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
        for round, witness := range n.Witnesses[addr] {
            if round >= n.FirstRoundOfFameUndecided[addr] {
                fameUndecidedWitnesses = append(fameUndecidedWitnesses, witness)
            }
        }
    }

    for _, e := range fameUndecidedWitnesses {
        // Get all witnesses that have greater rounds
        var witnessesWithGreaterRounds []*Event
        for addr := range n.Hashgraph {
            for round, witness := range n.Witnesses[addr] {
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
            superMajorityThreshold := 2.0 * float64(len(n.Hashgraph)) / 3.0
            if (majorityVote && float64(trueVotes) > superMajorityThreshold) || (!majorityVote && float64(falseVotes) > superMajorityThreshold) {
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
                        if z.Round < e.Round {
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

                    sort.Stable(timestamps) // returns timestamps sorted in increasing order
                    medianTimestamp := timestamps[int(math.Floor(float64(len(timestamps))/2.0))]
                    e.ConsensusTimestamp = medianTimestamp
                    e.Latency = time.Now().Sub(e.Timestamp) // Event's timestamp was set during it's creation
                    n.ConsensusEvents = append(n.ConsensusEvents, e)
                    n.FirstEventOfNotConsensusIndex[e.Owner]++ // !
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
    dpMap, ok := n.seeDPMemory[current.Signature]

    if !ok {
        dpMap = make(map[string]bool)
        n.seeDPMemory[current.Signature] = dpMap
    }

    cachedResult, ok := dpMap[target.Signature]

    if ok {
        return cachedResult
    }

    if current.Signature == target.Signature || (current.Round > target.Round && current.Owner == target.Owner) {
        dpMap[target.Signature] = true
        return true
    }
    if (current.Round < target.Round) || (current.IsWitness && current.Round == target.Round) || isInitial(current) {
        dpMap[target.Signature] = false
        return false
    }
    /* SPONGE >>>
       selfParent, ok := n.Events[current.SelfParentHash]
       otherParent, ok := n.Events[current.OtherParentHash]
       if selfParent == nil || otherParent == nil {
       	fmt.Printf("%+v\n", current)

       }
       SPONGE <<< */

    result := n.see(n.Events[current.SelfParentHash], target) || n.see(n.Events[current.OtherParentHash], target)
    dpMap[target.Signature] = result
    return result
}

// If we see the target, and we go through 2n/3 different nodes as we do that, we say we strongly see that target. This function is used for choosing the famous witness
func (n *Node) stronglySee(current *Event, target *Event) bool {
    latestAncestors := n.getLatestAncestorFromAllNodes(current, target.Round)
    count := 0
    for _, latestAncestor := range latestAncestors {
        if n.see(latestAncestor, target) {
            count++
        }
    }

    return float64(count) > (2.0 * float64(len(n.Hashgraph)) / 3.0)
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
            // Pop queue
            currentEvent = queue[0]
            queue[0] = nil
            queue = queue[1:]

            // Check if we have assigned an ancestor for this address yet
            currentAncestorFromOwner, ok := latestAncestors[currentEvent.Owner]

            if !ok {
                // if not, assign current
                latestAncestors[currentEvent.Owner] = currentEvent
            } else if currentEvent.Round > currentAncestorFromOwner.Round && currentEvent.Owner == currentAncestorFromOwner.Owner {
                latestAncestors[currentEvent.Owner] = currentEvent
            } else if (currentEvent.Round >= currentAncestorFromOwner.Round) && n.see(currentEvent, currentAncestorFromOwner) {
                // if we did, check if current event is higher up
                latestAncestors[currentEvent.Owner] = currentEvent
            }

            // If current event is initial, it has no parents, so we should not bother with next iteration
            if !isInitial(currentEvent) {
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

//isInitial : Returns true if given event is an initial event, false otherwise.
func isInitial(e *Event) bool {
    return e.SelfParentHash == "" || e.OtherParentHash == ""
}

//GenerateTransactions : Generates an arbitrary amount of random transactions
func (n *Node) GenerateTransactions(count int, max float64, min float64, peerAddress []string) []Transaction {
    // Prepare transactions
    transactions := make([]Transaction, count)
    for i := 0; i < count; i++ {
        randomPeerAddress := peerAddress[rand.Intn(len(peerAddress))]

        for randomPeerAddress == "" {
            randomPeerAddress = peerAddress[rand.Intn(len(peerAddress))]
        }

        randomAmount := min + rand.Float64()*(max-min)
        transactions[i] = Transaction{
            SenderAddress:   n.Address,
            ReceiverAddress: randomPeerAddress,
            Amount:          randomAmount,
        }
    }

    return transactions

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
