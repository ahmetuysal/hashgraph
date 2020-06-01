package main

import (
    "github.com/asticode/go-astikit"
    "github.com/asticode/go-astilectron"
    "log"
    "os"
)

func main()  {
    var a, _ = astilectron.New(log.New(os.Stderr, "", 0), astilectron.Options{
        AppName: "Hashgraph Demo",
        VersionAstilectron: "0.39.0",
        VersionElectron: "7.1.10",
    })
    defer a.Close()

    // Start astilectron
    a.Start()

    var w, _ = a.NewWindow("http://127.0.0.1:4000", &astilectron.WindowOptions{
        Center: astikit.BoolPtr(true),
        Height: astikit.IntPtr(600),
        Width:  astikit.IntPtr(600),
    })
    w.Create()

    // Open dev tools
    w.OpenDevTools()


    // Blocking pattern
    a.Wait()
}