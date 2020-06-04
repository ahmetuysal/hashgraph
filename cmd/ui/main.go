package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "github.com/asticode/go-astikit"
    "github.com/asticode/go-astilectron"
    bootstrap "github.com/asticode/go-astilectron-bootstrap"
    "log"
)

var (
    w *astilectron.Window
)

func main() {
    flag.Parse()

    // Create logger
    l := log.New(log.Writer(), log.Prefix(), log.Flags())

    bootstrap.Run(bootstrap.Options{
        AstilectronOptions: astilectron.Options{
            AppName:            "HashgraphDemo",
            SingleInstance:     true,
            VersionAstilectron: "0.39.0",
            VersionElectron:    "7.1.10",
        },
        Logger: l,
        MenuOptions: []*astilectron.MenuItemOptions{{
            Label: astikit.StrPtr("File"),
            SubMenu: []*astilectron.MenuItemOptions{
                {
                    Label: astikit.StrPtr("About"),
                    OnClick: func(e astilectron.Event) (deleteListener bool) {
                        if err := bootstrap.SendMessage(w, "about", "Hello!", func(m *bootstrap.MessageIn) {
                            // Unmarshal payload
                            var s string
                            if err := json.Unmarshal(m.Payload, &s); err != nil {
                                l.Println(fmt.Errorf("unmarshaling payload failed: %w", err))
                                return
                            }
                            l.Printf("About modal has been displayed and payload is %s!\n", s)
                        }); err != nil {
                            l.Println(fmt.Errorf("sending about event failed: %w", err))
                        }
                        return
                    },
                },
                {Role: astilectron.MenuItemRoleClose},
            },
        }},
        Windows: []*bootstrap.Window{{
            Homepage:       "index.html",
            MessageHandler: handleMessages,
            Options: &astilectron.WindowOptions{
                BackgroundColor: astikit.StrPtr("#fff"),
                Center:          astikit.BoolPtr(true),
                Height:          astikit.IntPtr(700),
                Width:           astikit.IntPtr(700),
            },
        }},
    })
}

func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {
    return
}
