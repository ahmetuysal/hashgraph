package main

import (
    "../../pkg/dledger"
    "flag"
    "github.com/asticode/go-astikit"
    "github.com/asticode/go-astilectron"
    bootstrap "github.com/asticode/go-astilectron-bootstrap"
    "log"
    "net"
    "sync"
    "time"
)

var (
    w *astilectron.Window
)

const (
    defaultPort  = "8080" // Use this default port if it isn't specified via command line arguments.
    updatePeriod = 50 * time.Millisecond
)

func main() {
    flag.Parse()

    // Create logger
    l := log.New(log.Writer(), log.Prefix(), log.Flags())

    var mut sync.Mutex
    mut.Lock()

    go bootstrap.Run(bootstrap.Options{
        AstilectronOptions: astilectron.Options{
            AppName:            "HashgraphDemo",
            SingleInstance:     true,
            VersionAstilectron: "0.39.0",
            VersionElectron:    "7.1.10",
        },
        Logger: l,
        OnWait: func(_ *astilectron.Astilectron, ws []*astilectron.Window, _ *astilectron.Menu, _ *astilectron.Tray, _ *astilectron.Menu) error {
            w = ws[0]
            mut.Unlock()
            return nil
        },
        Windows: []*bootstrap.Window{{
            Homepage:       "index.html",
            MessageHandler: handleMessages,
            Options: &astilectron.WindowOptions{
                BackgroundColor: astikit.StrPtr("#fff"),
                Center:          astikit.BoolPtr(true),
                Height:          astikit.IntPtr(736),
                Width:           astikit.IntPtr(1080),
            },
        }},
    })

    mut.Lock()

    port := defaultPort

    localAddr := getLocalAddress()

    peers := make(map[string]string)
    peers[localAddr+":8080"] = "Ahmet"
    peers[localAddr+":9090"] = "Erhan"
    peers[localAddr+":7070"] = "Öznur"
    peers[localAddr+":6060"] = "Waris"

    _ = bootstrap.SendMessage(w, "peers", peers)

    distributedLedger := dledger.NewDLedgerFromPeers(port, peers)

    distributedLedger.WaitForPeers()
    distributedLedger.Start()

    knownConsensusEvents := 0
    knownHashgraphEvents := make(map[string]int, len(peers)+1)
    firstRoundOfFameUndecided := make(map[string]uint32, len(peers)+1)
    firstRoundOfFameUndecided[distributedLedger.MyAddress] = 0
    for addr := range peers {
        firstRoundOfFameUndecided[addr] = 0
    }

    for {
        time.Sleep(updatePeriod)
        distributedLedger.Node.RWMutex.RLock()
        for addr := range distributedLedger.Node.Hashgraph {
            for _, event := range distributedLedger.Node.Hashgraph[addr][knownHashgraphEvents[addr]:] {
                _ = bootstrap.SendMessage(w, "event", event)

            }
            knownHashgraphEvents[addr] = len(distributedLedger.Node.Hashgraph[addr])
        }

        for _, newConsensusEvent := range distributedLedger.Node.ConsensusEvents[knownConsensusEvents:] {
            _ = bootstrap.SendMessage(w, "event", newConsensusEvent)
        }

        for addr, rofu := range distributedLedger.Node.FirstRoundOfFameUndecided {
            if rofu > firstRoundOfFameUndecided[addr] {
                for i := firstRoundOfFameUndecided[addr]; i < rofu; i++ {
                    witness, ok := distributedLedger.Node.Witnesses[addr][i]
                    if ok {
                        _ = bootstrap.SendMessage(w, "event", witness)
                    }
                }
                firstRoundOfFameUndecided[addr] = rofu
            }
        }

        knownConsensusEvents = len(distributedLedger.Node.ConsensusEvents)
        distributedLedger.Node.RWMutex.RUnlock()
    }

}

func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {
    return
}

func getLocalAddress() string {
    conn, err := net.Dial("udp", "eng.ku.edu.tr:80")
    handleError(err)
    defer func() {
        handleError(conn.Close())
    }()
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP.String()
}

func handleError(e error) {
    if e != nil {
        panic(e)
    }
}
