package main

import (
    "../../pkg/dledger"
    "flag"
    "github.com/asticode/go-astikit"
    "github.com/asticode/go-astilectron"
    bootstrap "github.com/asticode/go-astilectron-bootstrap"
    "log"

)

var (
    w *astilectron.Window
)

const (
    defaultPort = "8080" // Use this default port if it isn't specified via command line arguments.
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

    port := defaultPort

    peers := make(map[string]string)
    peers["localhost:8080"] = "Alice"
    peers["localhost:9090"] = "Bob"
    peers["localhost:7070"] = "Carol"
    peers["localhost:6060"] = "Dave"

    distributedLedger := dledger.NewDLedgerFromPeers(port, peers)

    distributedLedger.WaitForPeers()

}

func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {
    return
}
