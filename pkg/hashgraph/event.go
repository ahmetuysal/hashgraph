package hashgraph

//Event : An event of hashgraph
type Event struct {
	Owner           string         // Address of the node that created this event
	Signature       string         // Event should be signed by it's creator
	SelfParentHash  string         // Hash of the self-parent, which is the hash for the event before this event in my timeline.
	OtherParentHash string         // Hash of the other-parent, which is the hash for the last event of the peer that called me.
	Timestamp       int            // Should we use vector clocks for this? Or, what to the authors use?
	Transactions    *[]interface{} // List of transactions for this event, size can be 0 too.
	Round           uint32         // Calculated by divideRounds(),  initial event is round 1
	IsWitness       bool           // Is this event the first event in it's round at it's member?
}
