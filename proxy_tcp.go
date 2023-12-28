package kish

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"

	"github.com/hashicorp/yamux"
)

func (rs *KishServer) runTcp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t := extractBearerToken(r.Header.Get("Authorization"))
	if err := validateToken(t, rs.TokenSet); err != nil {
		log.Printf("validateToken failed: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !rs.EnableTCPForwarding {
		w.Header().Set("X-Error-Message", "TCP forwarding is not enabled")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Print("net.Listen:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer listener.Close()

	respHeader := http.Header{}
	respHeader.Set("X-Kish-URL", "tcp://"+listener.Addr().String())

	c, err := websocketUpgrader.Upgrade(w, r, respHeader)
	if err != nil {
		log.Print("upgrader.Upgrade:", err)
		return
	}
	defer c.Close()

	session, err := yamux.Server(MakeRWC(c), nil)
	if err != nil {
		log.Print("yamux.Server:", err)
		return
	}
	defer session.Close()
	go func() {
		<-session.CloseChan()
		log.Print("yamux.Session closed")
		cancel()
	}()
	go forwardFromNetListenerToYamuxSession(listener, session)
	<-ctx.Done()
}

func forwardFromNetListenerToYamuxSession(listener net.Listener, session *yamux.Session) {
	for {
		clientConn, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				// 正常終了とみなす
				return
			}
			log.Printf("error Accept: %s", err)
			return
		}
		go func() {
			defer clientConn.Close()
			serverConn, err := session.Open()
			if err != nil {
				log.Printf("error session.Open: %s", err)
				return
			}
			defer serverConn.Close()
			err = Passthrough(clientConn, serverConn)
			if err != nil {
				log.Printf("error Passthrough: %s", err)
				return
			}
		}()
	}
}
