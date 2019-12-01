/**
 * WSSH Demo
 *
 * @abstract  Demo
 * @version   1.0.0
 * @author    Wildlife <admin@lanseyujie.com>
 * @link      https://lanseyujie.com
 */

package main

import (
    "flag"
    "github.com/lanseyujie/wssh/wssh"
    "golang.org/x/net/websocket"
    "io/ioutil"
    "log"
    "net/http"
    "os"
)

var (
    user     string
    host     string
    port     uint
    key      string
    password string
    help     bool
)

func main() {
    var shell *wssh.WebSocketShell

    flag.StringVar(&user, "u", "root", "ssh user")
    flag.StringVar(&host, "h", "localhost", "ssh host")
    flag.UintVar(&port, "P", 22, "ssh port")
    flag.StringVar(&key, "k", "", "private key file path")
    flag.StringVar(&password, "p", "", "ssh or private key password")
    flag.BoolVar(&help, "help", false, "this help")

    flag.Parse()

    if help {
        flag.PrintDefaults()
        os.Exit(0)
    }

    log.SetPrefix("[ERROR] ")

    if len(key) > 0 {
        // get private key.
        key, err := ioutil.ReadFile(key)
        if err != nil {
            log.Fatalln(err)
        }
        shell = wssh.NewWebSocketShell(host, int(port), user, password, key)
    } else {
        shell = wssh.NewWebSocketShell(host, int(port), user, password, nil)
    }

    // test config
    err := shell.Connect()
    if err != nil {
        log.Fatalln("ssh config", err)
    } else {
        shell.Close()
    }

    http.Handle("/", http.FileServer(http.Dir("./static/")))

    http.Handle("/ssh", websocket.Handler(shell.WebSocket))

    err = http.ListenAndServe(":8080", nil)
    log.Fatalln(err)
}
