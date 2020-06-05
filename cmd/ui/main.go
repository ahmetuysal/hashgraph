package main

import (
    "../../pkg/dledger"
    "flag"
    "github.com/asticode/go-astikit"
    "github.com/asticode/go-astilectron"
    bootstrap "github.com/asticode/go-astilectron-bootstrap"
    "log"
    "net"
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
            return nil
        },
        Windows: []*bootstrap.Window{{
            Homepage:       "index.html",
            MessageHandler: handleMessages,
            Options: &astilectron.WindowOptions{
                BackgroundColor: astikit.StrPtr("#fff"),
                Center:          astikit.BoolPtr(true),
                Height:          astikit.IntPtr(720),
                Width:           astikit.IntPtr(1080),
            },
        }},
    })

    time.Sleep(5 * time.Second)
    w.OpenDevTools()
    port := defaultPort

    localAddr := getLocalAddress()

    peers := make(map[string]string)
    peers[localAddr+":8080"] = "Alice"
    peers[localAddr+":9090"] = "Bob"

    distributedLedger := dledger.NewDLedgerFromPeers(port, peers)

    distributedLedger.WaitForPeers()
    distributedLedger.Start()

    knownConsensusEvents := 0

    for {
        time.Sleep(updatePeriod)
        distributedLedger.Node.RWMutex.RLock()
        for _, newConsensusEvent := range distributedLedger.Node.ConsensusEvents[knownConsensusEvents:] {
            _ = bootstrap.SendMessage(w, "event", newConsensusEvent)
        }
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
