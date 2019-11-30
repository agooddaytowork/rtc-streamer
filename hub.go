package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v2"
)

type Wrap struct {
	*webrtc.DataChannel
}

var pc *webrtc.PeerConnection

func (rtc *Wrap) Write(data []byte) (int, error) {
	err := rtc.DataChannel.Send(data)
	return len(data), err
}

func hub(ws *websocket.Conn, conf Config) {
	var msg Session
	for {
		err := ws.ReadJSON(&msg)
		if err != nil {
			_, ok := err.(*websocket.CloseError)
			if !ok {
				log.Println("websocket", err)
			}
			break
		}
		err = startRTC(ws, msg, conf)
		if err != nil {
			log.Println(err)
		}
	}
}

func startRTC(ws *websocket.Conn, data Session, conf Config) error {
	if data.Error != "" {
		return fmt.Errorf(data.Error)
	}

	switch data.Type {
	case "signal_OK":
		log.Println("Signal OK")
	case "offer":
		var err error
		pc, err = webrtc.NewPeerConnection(configRTC)
		if err != nil {
			return err
		}

		pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
			log.Println("ICE Connection State has changed:", state.String())

			if state.String() == "failed" {
				pc.Close()
			}
		})

		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			if dc.Label() == "SSH" {
				ssh, err := net.Dial("tcp", fmt.Sprintf("%s:%d", conf.Host, conf.Port))
				if err != nil {
					log.Println("ssh dial failed:", err)
					pc.Close()
				} else {
					log.Println("Connect SSH socket")
					DataChannel(dc, ssh)
				}
			} else if dc.Label() == "videostream" {
				log.Println("Connect To Video stream")
				VideoStreamChannel(dc)
			}
		})

		if err := pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  data.Sdp,
		}); err != nil {
			pc.Close()
			return err
		}

		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			pc.Close()
			return err
		}

		err = pc.SetLocalDescription(answer)
		if err != nil {
			pc.Close()
			return err
		}

		if err = ws.WriteJSON(answer); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown signaling message %v", data.Type)
	}
	return nil
}

func DataChannel(dc *webrtc.DataChannel, ssh net.Conn) {
	dc.OnOpen(func() {

		err := dc.SendText("OPEN_RTC_CHANNEL")
		if err != nil {
			log.Println("write data error:", err)
		}
		io.Copy(&Wrap{dc}, ssh)
	})
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		ssh.Write(msg.Data)
	})
	dc.OnClose(func() {
		log.Printf("Close SSH socket")
		ssh.Close()
	})
}

func VideoStreamChannel(dc *webrtc.DataChannel) {

	isClose := false
	dc.OnOpen(func() {
		// nBytes, nChunks := int64(0), int64(0)
		// r := bufio.NewReader(os.Stdin)
		// buf := make([]byte, 0, 8*1024)

		// for {
		// 	n, err := r.Read(buf[:cap(buf)])
		// 	buf = buf[:n]
		// 	if n == 0 {
		// 		if err == nil {
		// 			continue
		// 		}
		// 		if err == io.EOF {
		// 			break
		// 		}
		// 		log.Fatal(err)
		// 	}
		// 	nChunks++
		// 	nBytes += int64(len(buf))
		// 	// process buf
		// 	if err != nil && err != io.EOF {
		// 		log.Fatal(err)
		// 	}

		// 	dc.Send(buf)
		// 	// log.Println(len(buf))
		// }

		buf := make([]byte, 1024)

		for !isClose {
			var nBytes int
			var err error
			nBytes, err = os.Stdin.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Read error: %s\n", err)
				}
				break
			}

			err = dc.Send(buf[0:nBytes])
			if err != nil {
				// log.Fatalf("Write error: %s\n", err)

				dc.Close()
				log.Printf("Write error: %s ", err)

			}

		}
		log.Printf("Exit reading loop")

		defer fmt.Println("thread end")

		// log.Println("Bytes:", nBytes, "Chunks:", nChunks)
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println(msg.Data)
	})

	dc.OnClose(func() {
		isClose = true
		log.Printf("Close Video Channel")
	})
}
