package main

import (
	"fmt"
	"io"
	"log"
	"net"

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
				// pc.Close()
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
				VideoStreamChannel(dc, func() {

					log.Println("close data channel callback ")
					pc.Close()

				})
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

func VideoStreamChannel(dc *webrtc.DataChannel, closeChannelCB func()) {

	dc.OnOpen(func() {

		theDataChannel := make(chan []byte)
		TheReader.AddListener("newdata", theDataChannel)

		var err error
		for {

			msg := <-theDataChannel

			err = dc.Send(msg)

			if err != nil {
				// log.Fatalf("Write error: %s\n", err)
				// log.Printf("Write error: %s ", err)
				// dc.Close()
				// closeChannelCB()

				TheReader.RemoveListener("newdata", theDataChannel)

			}
		}

	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Println(msg.Data)
	})

	dc.OnClose(func() {

		log.Printf("Close Video Channel")
	})
}
