package hashgraph

type Event struct {
    signature       string         // Event should be signed by it's creator
    selfParentHash  string         // Hash of the self-parent, which is the hash for the event before this event in my timeline.
    otherParentHash string         // Hash of the other-parent, which is the hash for the last event of the peer that called me.
    timestamp       int            // Should we use vector clocks for this? Or, what to the authors use?
    transactions    *[]interface{} // List of transactions for this event, size can be 0 too.
}

