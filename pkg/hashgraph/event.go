package hashgraph

type Event struct {
    Signature       string         // Event should be signed by it's creator
    SelfParentHash  string         // Hash of the self-parent, which is the hash for the event before this event in my timeline.
    OtherParentHash string         // Hash of the other-parent, which is the hash for the last event of the peer that called me.
    Timestamp       int            // Should we use vector clocks for this? Or, what to the authors use?
    Transactions    *[]interface{} // List of transactions for this event, size can be 0 too.
}

