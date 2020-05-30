package hashgraph

import "time"

type Node struct {
    Address   string             // ip:port of the peer
    Hashgraph map[string][]Event // local copy of hashgraph, i.e, events all nodes
}

type SyncEventsDTO struct {
    SenderAddress string
    MissingEvents map[string][]Event
}

func (n *Node) GetNumberOfMissingEvents(numEventsAlreadyKnown map[string]int, numEventsToSend *map[string]int) error {
    for addr := range n.Hashgraph {
        (*numEventsToSend)[addr] = numEventsAlreadyKnown[addr] - len(n.Hashgraph[addr])
    }
    return nil
}

func (n *Node) SyncAllEvents(events SyncEventsDTO, success *bool) error {
    for addr := range events.MissingEvents {
        n.Hashgraph[addr] = append(n.Hashgraph[addr], events.MissingEvents[addr]...)
    }

    // TODO: create new event
    newEvent := Event{
        Signature:       time.Now().String(),
        SelfParentHash:  n.Hashgraph[n.Address][len(n.Hashgraph[n.Address])-1].Signature,
        OtherParentHash: n.Hashgraph[events.SenderAddress][len(n.Hashgraph[events.SenderAddress])-1].Signature,
        Timestamp:       0,
        Transactions:    nil,
    }

    n.Hashgraph[n.Address] = append(n.Hashgraph[n.Address], newEvent)

    n.DivideRounds()
    n.DecideFame()
    n.FindOrder()

    return nil
}

func (n Node) DivideRounds() {

}

func (n Node) DecideFame() {

}

func (n Node) FindOrder() {

}
