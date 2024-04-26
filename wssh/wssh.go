package wssh

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

// msg flag type.

type MsgType byte

const (
	UNKNOWN MsgType = iota
	CONFIG
	SESSION
	RESIZE
	HEARTBEAT
)

const bufferSize = 4096

// WindowResize terminal window resize.
type WindowResize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

type WebSocketShell struct {
	opts options
	pool *sync.Pool
}

func New(opts ...Option) *WebSocketShell {
	o := options{
		user:         "root",
		host:         "localhost",
		port:         22,
		identityFile: "",
		password:     "",
		listenPort:   8022,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return &WebSocketShell{
		opts: o,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, bufferSize)
			},
		},
	}
}

func (w *WebSocketShell) Open() (*ssh.Client, error) {
	var auth []ssh.AuthMethod

	{
		if len(w.opts.identityFile) > 0 {
			priKey, err := os.ReadFile(w.opts.identityFile)
			if err != nil {
				return nil, err
			}

			var signer ssh.Signer
			if len(w.opts.password) > 0 {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(priKey, []byte(w.opts.password))
			} else {
				signer, err = ssh.ParsePrivateKey(priKey)
			}
			if err != nil {
				return nil, err
			}
			auth = append(auth, ssh.PublicKeys(signer))
		} else if len(w.opts.password) > 0 {
			auth = append(auth, ssh.Password(w.opts.password))
		}
	}

	client, err := ssh.Dial("tcp", w.opts.host+":"+strconv.Itoa(w.opts.port), &ssh.ClientConfig{
		User:            w.opts.user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		BannerCallback:  ssh.BannerDisplayStderr(),
		ClientVersion:   "SSH-2.0-WSSH",
		Timeout:         10 * time.Second,
	})

	return client, err
}

// RequestPty .
func requestPty(sess *ssh.Session, cols, rows int) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // enable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4 kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4 kbaud
	}

	// request pseudo terminal.
	err := sess.RequestPty("xterm-256color", rows, cols, modes)

	return err
}

func (w *WebSocketShell) Send(ws *websocket.Conn, reader io.Reader) (err error) {
	buf := w.pool.Get().([]byte)
	defer func() {
		buf = buf[:0]
		w.pool.Put(buf)
	}()

	buf = make([]byte, cap(buf))
	buf[0] = byte(SESSION)
	_, err = reader.Read(buf[1:])
	if err != nil {
		return
	}

	err = websocket.Message.Send(ws, buf)

	return
}

func (w *WebSocketShell) Receive(ws *websocket.Conn, session *ssh.Session, writer io.WriteCloser) (err error) {
	buf := w.pool.Get().([]byte)
	defer func() {
		buf = buf[:0]
		w.pool.Put(buf)
	}()

	err = websocket.Message.Receive(ws, &buf)
	if err != nil {
		return
	}

	switch MsgType(buf[0]) {
	case SESSION:
		_, err = writer.Write(buf[1:])
	case RESIZE:
		resize := WindowResize{}
		err = json.Unmarshal(buf[1:], &resize)
		if err != nil {
			return
		}

		err = session.WindowChange(resize.Rows, resize.Cols)
	case HEARTBEAT:
		if string(buf[1:]) == "ping" {
			err = websocket.Message.Send(ws, []byte{byte(HEARTBEAT), 'p', 'o', 'n', 'g'})
		}
	default:
		err = errors.New("[ERROR] unexpected msg type")
	}

	return
}

func (w *WebSocketShell) WebSocket(ws *websocket.Conn) {
	defer ws.Close()

	var client *ssh.Client
	var session *ssh.Session
	{
		var err error
		if client, err = w.Open(); err != nil {
			log.Println("[ERROR] ssh dial err:", err)
			return
		}
		defer client.Close()

		if session, err = client.NewSession(); err != nil {
			log.Println("[ERROR] new session err:", err)
			return
		}
		defer session.Close()

		if err = requestPty(session, 80, 30); err != nil {
			log.Println("[ERROR] request pty err:", err)
			return
		}
	}

	// set io.Reader and io.Writer from terminal session.
	var reader io.Reader
	var writer io.WriteCloser
	{
		stdout, err := session.StdoutPipe()
		if err != nil {
			log.Println("[ERROR] session stdout pipe:", err)
			return
		}
		stderr, err := session.StdoutPipe()
		if err != nil {
			log.Println("[ERROR] session stdout pipe:", err)
			return
		}

		reader = io.MultiReader(stdout, stderr)

		writer, err = session.StdinPipe()
		if err != nil {
			log.Println("[ERROR] session stdin pipe:", err)
			return
		}
		defer writer.Close()
	}

	stopCh := make(chan struct{})
	go func() {
		select {
		case <-stopCh:
			session.Close()
			log.Println("[ERROR] wssh disconnected")
		}
	}()

	// read from terminal and write to frontend.
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			err := w.Send(ws, reader)
			if err != nil {
				if err == io.EOF {
					select {
					case <-stopCh:
						return
					default:
						close(stopCh)
					}

					return
				}
				log.Println("[ERROR] websocket message send:", err)
			}
		}
	}()

	// read from frontend and write to terminal.
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			err := w.Receive(ws, session, writer)
			if err != nil {
				if err == io.EOF || errors.Is(err, net.ErrClosed) {
					select {
					case <-stopCh:
						return
					default:
						close(stopCh)
					}

					return
				} else {
					log.Println("[ERROR] websocket message receive:", err)
				}
			}
		}
	}()

	err := session.Shell()
	if err != nil {
		log.Println("[ERROR] session shell: ", err)
		return
	}

	err = session.Wait()
	if err != nil {
		log.Println("[ERROR] session wait: ", err)
		return
	}
}
