package main

import "../../pkg/hashgraph"

func main() {
    myNode := hashgraph.Node{
        Address:   "",
        Hashgraph: nil,
    }

    go hashgraphMain(myNode)
}

func hashgraphMain(node hashgraph.Node) {
    for {
        // TODO: select a node at random
        // TODO: sync all events with that node
        // TODO: create a new event
        node.DivideRounds()
        node.DecideFame()
        node.FindOrder()
    }
}
