package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/websocket"

	"github.com/lanseyujie/wssh/static"
	"github.com/lanseyujie/wssh/wssh"
)

var (
	user         string
	host         string
	port         int
	identityFile string
	password     string
	listenPort   int
	help         bool
)

func main() {
	{
		flag.StringVar(&user, "u", "root", "ssh user")
		flag.StringVar(&host, "h", "localhost", "ssh host")
		flag.IntVar(&port, "P", 22333, "ssh port")
		flag.StringVar(&identityFile, "i", "", "private key file path")
		flag.StringVar(&password, "p", "", "ssh or private key password")
		flag.IntVar(&listenPort, "l", 8022, "web listen port")
		flag.BoolVar(&help, "help", false, "this help")
		flag.Parse()
		if help {
			flag.PrintDefaults()
			return
		}
	}

	shell := wssh.New(
		wssh.User(user), wssh.Host(host), wssh.Port(port),
		wssh.IdentityFile(identityFile), wssh.Password(password), wssh.ListenPort(port),
	)

	http.Handle("/", http.FileServer(http.FS(static.View)))
	http.Handle("/ssh", websocket.Handler(shell.WebSocket))

	log.Printf("Websocket Shell: http://127.0.0.1:%v", listenPort)
	err := http.ListenAndServe(fmt.Sprintf(":%v", listenPort), nil)

	log.Fatalln(err)
}
