package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/no2a/kish"
	"github.com/spf13/cobra"
)

var flag_tcpTarget string

func tcpParseArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("wrong number of argments")
	}
	flag_tcpTarget := args[0]
	flag_tcpTarget = canonicalizeTargetArg(flag_tcpTarget)
	if flag_tcpTarget == "" {
		return fmt.Errorf("target `%s` is invalid", flag_tcpTarget)
	}
	return nil
}

func tcpMain(cmd *cobra.Command, args []string) {
	wsConn, proxyURL, header := dialKish("proxy1")
	fmt.Printf("%s -> %s\n", proxyURL, flag_tcpTarget)
	fmt.Printf("Allow IP: %s\n", header.Get("X-Kish-Allow-IP"))
	err := tcpRun(kish.MakeRWC(wsConn), flag_tcpTarget)
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
