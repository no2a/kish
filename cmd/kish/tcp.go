package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/no2a/kish"
)

func tcpMain() {
	target := canonicalizeTargetArg(*flag_tcpTarget)
	wsConn, proxyURL, header := dialKish("proxy1")
	tuiWriteText(fmt.Sprintf("%s -> %s\n", proxyURL, target))
	tuiWriteText(fmt.Sprintf("Allow IP: %s\n", header.Get("X-Kish-Allow-IP")))
	err := tcpRun(kish.MakeRWC(wsConn), target)
	if err != nil {
		log.Fatal(err)
	}
}

func tcpForwardToTarget(clientConn io.ReadWriteCloser, target string) {
	defer clientConn.Close()
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}
	defer targetConn.Close()
	err = kish.Passthrough(clientConn, targetConn)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}
}

func tcpRun(conn io.ReadWriteCloser, target string) error {
	defer conn.Close()
	session, err := yamux.Client(conn, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	for {
		clientConn, err := session.Accept()
		if err != nil {
			return err
		}
		go tcpForwardToTarget(clientConn, target)
	}
}
