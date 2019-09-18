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
    "golang.org/x/net/websocket"
    "io/ioutil"
    "log"
    "net/http"
    "wssh/wssh"
)

func main() {
    // get private key.
    key, err := ioutil.ReadFile("/root/.ssh/id_rsa")
    if err != nil {
        log.Fatalln("unable to read private key: %v", err)
    }
    shell := wssh.NewWebSocketShell("192.168.1.10", 22, "root", "private_key_password", key)

    // or
    //shell := wssh.NewWebSocketShell("192.168.1.10", 22, "root", "ssh_password", nil)

    http.Handle("/", http.FileServer(http.Dir("./static/")))

    http.Handle("/ssh", websocket.Handler(shell.WebSocket))

    err = http.ListenAndServe(":8080", nil)
    log.Fatalln(err)
}
