package hashgraph

type Node struct {
    Address   string             // ip:port of the peer
    Hashgraph map[string][]Event // local copy of hashgraph, i.e, events all nodes
}

func (n *Node) GetNumberOfMissingEvents(numEventsAlreadyKnown map[string]int, numEventsToSend *map[string]int) error {

    return nil
}

func (n *Node) SyncAllEvents(missingEvents map[string][]Event, success *bool) error {

    return nil
}

func (n Node) DivideRounds() {

}

func (n Node) DecideFame() {

}

func (n Node) FindOrder() {

}