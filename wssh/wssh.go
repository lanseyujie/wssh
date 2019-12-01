/**
 * WSSH
 *
 * @abstract  WebSocketShell
 * @version   1.0.0
 * @author    Wildlife <admin@lanseyujie.com>
 * @link      https://lanseyujie.com
 */

package wssh

import (
    "crypto/x509"
    "encoding/json"
    "encoding/pem"
    "errors"
    "golang.org/x/crypto/ssh"
    "golang.org/x/net/websocket"
    "log"
    "net"
    "strconv"
    "time"
)

type WebSocketShell struct {
    Host     string
    Port     int
    Username string
    Password string
    Key      []byte
    Client   *ssh.Client
    Session  *ssh.Session
}

// msg flag type.
const (
    Terminal = iota
    Resize
    Heartbeat
)

// terminal window resize.
type WindowResize struct {
    Cols int `json:"cols"`
    Rows int `json:"rows"`
}

func NewWebSocketShell(host string, port int, username, password string, key []byte) *WebSocketShell {
    return &WebSocketShell{
        Host:     host,
        Port:     port,
        Username: username,
        Password: password,
        Key:      key,
        Client:   nil,
        Session:  nil,
    }
}

// connect to the ssh.
func (wssh *WebSocketShell) Connect() error {
    var err error
    var auth []ssh.AuthMethod
    var signer ssh.Signer

    if len(wssh.Key) > 0 {
        if len(wssh.Password) > 0 {
            block, rest := pem.Decode(wssh.Key)
            if len(rest) > 0 {
                return errors.New("extra data included in key")
            }

            der, err := x509.DecryptPEMBlock(block, []byte(wssh.Password))
            if err != nil {
                return err
            }
            key, err := x509.ParsePKCS1PrivateKey(der)
            if err != nil {
                return err
            }

            signer, err = ssh.NewSignerFromKey(key)
        } else {
            // create the signer for this private key.
            signer, err = ssh.ParsePrivateKey(wssh.Key)
        }

        if err != nil {
            return err
        }

        auth = []ssh.AuthMethod{
            // use the public keys method for remote authentication.
            ssh.PublicKeys(signer),
        }
    } else {
        auth = []ssh.AuthMethod{
            ssh.Password(wssh.Password),
        }
    }

    // get auth method.
    config := &ssh.ClientConfig{
        User:    wssh.Username,
        Auth:    auth,
        Timeout: 30 * time.Second,
        HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
            return nil
        },
    }

    // connect to ths ssh.
    wssh.Client, err = ssh.Dial("tcp", wssh.Host+":"+strconv.Itoa(wssh.Port), config)
    if err != nil {
        return err
    }

    // create session.
    wssh.Session, err = wssh.Client.NewSession()

    return err
}

// config the terminal modes.
func (wssh *WebSocketShell) Config(cols, rows int) error {
    modes := ssh.TerminalModes{
        ssh.ECHO:          1,     // enable echoing
        ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4 kbaud
        ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4 kbaud
    }

    // request pseudo terminal.
    err := wssh.Session.RequestPty("xterm-256color", rows, cols, modes)

    return err
}

func (wssh *WebSocketShell) Close() {
    if wssh.Session != nil {
        _ = wssh.Session.Close()
    }

    if wssh.Client != nil {
        _ = wssh.Client.Close()
    }
}

func (wssh *WebSocketShell) WebSocket(ws *websocket.Conn) {
    err := wssh.Connect()
    if err != nil {
        log.Fatalln(err)
    }

    err = wssh.Config(80, 30)
    if err != nil {
        log.Fatalln(err)
    }

    // set io.Reader and io.Writer from terminal session.
    sshReader, err := wssh.Session.StdoutPipe()
    if err != nil {
        log.Fatalln(err)
    }

    sshWriter, err := wssh.Session.StdinPipe()
    if err != nil {
        log.Fatalln(err)
    }

    // read from terminal and write to frontend.
    go func() {
        defer func() {
            _ = ws.Close()
            wssh.Close()
        }()

        for {
            buf := make([]byte, 4096)
            buf[0] = Terminal
            _, err := sshReader.Read(buf[1:])
            if err != nil {
                log.Println(err)
                return
            }

            // send binary frame.
            err = websocket.Message.Send(ws, buf)
            if err != nil {
                log.Println(err)
                return
            }
        }
    }()

    // read from frontend and write to terminal.
    go func() {
        defer func() {
            _ = ws.Close()
            wssh.Close()
        }()

        for {
            // receive binary frame.
            var buf []byte
            err := websocket.Message.Receive(ws, &buf)
            if err != nil {
                log.Println(err)
                return
            }

            switch buf[0] {
            case Terminal:
                _, err = sshWriter.Write(buf[1:])
            case Resize:
                resize := WindowResize{}
                err = json.Unmarshal(buf[1:], &resize)
                if err != nil {
                    log.Println(err)
                    return
                }

                err = wssh.Session.WindowChange(resize.Rows, resize.Cols)
            case Heartbeat:
                if string(buf[1:]) == "ping" {
                    err = websocket.Message.Send(ws, []byte{2, 'p', 'o', 'n', 'g'})
                }
            default:
                log.Println("Unexpected data type")
            }

            if err != nil {
                log.Println(err)
                return
            }
        }
    }()

    // start remote shell.
    err = wssh.Session.Shell()
    if err != nil {
        log.Println("failed to start shell: ", err)
    }

    err = wssh.Session.Wait()
    if err != nil {
        log.Println("failed to wait shell: ", err)
    }
}
