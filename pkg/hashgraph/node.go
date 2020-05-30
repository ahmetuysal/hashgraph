package hashgraph

type Node struct {
    Address   string             // ip:port of the peer
    Hashgraph map[string][]Event // local copy of hashgraph, i.e, events all nodes
}

type SyncEventsDTO struct {
    SenderAddress string
    MissingEvents map[string][]Event
}

func (n *Node) GetNumberOfMissingEvents(numEventsAlreadyKnown map[string]int, numEventsToSend *map[string]int) error {

    return nil
}

func (n *Node) SyncAllEvents(events SyncEventsDTO, success *bool) error {

    return nil
}

func (n Node) DivideRounds() {

}

func (n Node) DecideFame() {

}

func (n Node) FindOrder() {

}
