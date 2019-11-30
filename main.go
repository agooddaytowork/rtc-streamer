package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"bufio"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	ini "github.com/pierrec/go-ini"
)

type Config struct {
	Uuid string `ini:"uuid,identify"`
	Host string `ini:"host,ssh"`
	Port int    `ini:"port,ssh"`
}

type Session struct {
	Type  string `json:"type"`
	Sdp   string `json:"sdp"`
	Error string `json:"error"`
}

var TheReader ReadStream

func reconnect(query string) *websocket.Conn {
	var u url.URL
	var ws *websocket.Conn
	var err error

	u = url.URL{Scheme: "wss", Host: "stublab.io:8808", Path: "/signal", RawQuery: query}
	for {
		ws, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Println(err)
			time.Sleep(30 * time.Second)
			continue
		}
		break
	}

	lastResponse := time.Now()
	ws.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		for {
			err := ws.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				log.Println(err)
				return
			}
			time.Sleep((120 * time.Second) / 2)
			if time.Now().Sub(lastResponse) > (120 * time.Second) {
				log.Println("Signaling server close connection")
				ws.Close()
				return
			}
		}
	}()

	return ws
}

func main() {
	log.SetFlags(0)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	var executablePath string
	var conf Config
	save := false

	ex, err := os.Executable()
	check(err)
	executablePath = filepath.Dir(ex)

	newkey := flag.Bool("newkey", false, "new generate key uuid of connection")
	getkey := flag.Bool("getkey", false, "display key uuid")
	setport := flag.Int("port", defaultPort, "port SSH server connection")
	sethost := flag.String("host", defaultHost, "address host SSH server connection")
	flag.Parse()

	loadConf := func() {
		file, err := os.Open(executablePath + "/config.ini")
		if err == nil {
			err = ini.Decode(file, &conf)
			check(err)
		}
		if os.IsNotExist(err) && !*newkey {
			log.Println("File config.ini not found, using option '-newkey'")
			os.Exit(0)
		}
		defer file.Close()
	}
	saveConf := func() {
		if conf.Host == "" {
			conf.Host = defaultHost
		}
		if conf.Port == 0 {
			conf.Port = defaultPort
		}
		file, err := os.Create(executablePath + "/config.ini")
		check(err)
		defer file.Close()
		err = ini.Encode(file, &conf)
		check(err)
	}

	loadConf()
	if *setport != defaultPort {
		conf.Port = *setport
		save = true
	}
	if *sethost != defaultHost {
		conf.Host = *sethost
		save = true
	}
	if *newkey {
		conf.Uuid = uuid.New().String()
		save = true
		fmt.Println("uuid:", conf.Uuid)
	}
	if save {
		saveConf()
	}
	if *getkey {
		fmt.Println("uuid:", conf.Uuid)
	}

	done := make(chan struct{})
	var ws *websocket.Conn

	go func() {
		for {
			select {
			case <-done:
				return
			case <-interrupt:
				// Cleanly close the connection by sending a close message and then
				// waiting (with timeout) for the server to close the connection.
				fmt.Println("interrupt")
				err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Println("write close:", err)
				}
				select {
				case <-done:
				case <-time.After(time.Second):
				}
				os.Exit(0)
			}
		}
	}()

	go func() {
		fmt.Println("Reading thread start ")
		// var err error

		nBytes, nChunks := int64(0), int64(0)
		r := bufio.NewReader(os.Stdin)
		// buf := make([]byte, 0, 64*1024)
		buf := make([]byte, 0, 512*1024)

		for {
			n, err := r.Read(buf[:cap(buf)])
			buf = buf[:n]
			if n == 0 {
				if err == nil {
					continue
				}
				if err == io.EOF {
					break
				}
				log.Fatal(err)
			}
			nChunks++
			nBytes += int64(len(buf))
			// process buf
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}

			TheReader.Emit("newdata", buf)
			// log.Println(len(buf))
		}

		// for {

		// 	nBytes, err = os.Stdin.Read(buf)
		// 	if err != nil {
		// 		if err != io.EOF {
		// 			log.Printf("Read error: %s\n", err)
		// 		}
		// 		break
		// 	}

		// 	TheReader.Emit("newdata", buf[0:nBytes])
		// err = dc.Send(buf[0:nBytes])
		// if err != nil {
		// 	// log.Fatalf("Write error: %s\n", err)

		// 	log.Printf("Write error: %s ", err)

		// }

		// }
		log.Printf("Exit reading loop")

		defer fmt.Println("thread end")
	}()

	for {
		query := "localUser=" + conf.Uuid
		ws = reconnect(query)
		hub(ws, conf)
		time.Sleep(30 * time.Second)
		log.Println("Reconnect with the signaling server")
	}

}

func check(e error) {
	if e != nil {
		log.Println(e)
	}
}
