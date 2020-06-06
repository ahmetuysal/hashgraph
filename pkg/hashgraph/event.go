package hashgraph

import "time"

//Event : An event of hashgraph
type Event struct {
	Owner              string        `json:"owner"`               // Address of the node that created this event
	Signature          string        `json:"signature"`           // Event should be signed by it's creator
	SelfParentHash     string        `json:"self_parent_hash"`    // Hash of the self-parent, which is the hash for the event before this event in my timeline.
	OtherParentHash    string        `json:"other_parent_hash"`   // Hash of the other-parent, which is the hash for the last event of the peer that called me.
	Timestamp          time.Time     `json:"timestamp"`           // Datetime of creation
	Transactions       []Transaction `json:"transactions"`        // List of transactions for this event, size can be 0 too.
	Round              uint32        `json:"round"`               // Calculated by divideRounds(),  initial event is round 1
	IsWitness          bool          `json:"is_witness"`          // Is this event the first event in it's round at it's member?
	IsFamous           bool          `json:"is_famous"`           // Is this witness a famous witness?
	IsFameDecided      bool          `json:"is_fame_decided"`     // Has there been a decision for this Event's fame?
	RoundReceived      uint32        `json:"round_received"`      // Consensus round
	ConsensusTimestamp time.Time     `json:"consensus_timestamp"` // Timestamp assigned by the consensus
	Latency            time.Duration `json:"latency"`             // How long did it take for this event to reach to a consensus
}
