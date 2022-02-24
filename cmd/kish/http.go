package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/yamux"
	"github.com/no2a/kish"
)

type KishClientHTTP struct {
	proxyURL         string
	target           string
	hostHeader       string
	originHeader     string
	locationHeaderSH *url.URL
	refererHeaderSH  *url.URL
}

func httpMain() {
	target := canonicalizeTargetArg(*flag_httpTarget)
	wsConn, proxyURL, header := dialKish("proxy2")
	tuiWriteText(fmt.Sprintf("%s -> %s\n", proxyURL, target))
	tuiWriteText(fmt.Sprintf("Allow IP: %s\n", header.Get("X-Kish-Allow-IP")))
	kc := KishClientHTTP{
		proxyURL:   proxyURL,
		target:     target,
		hostHeader: *flag_hostHeader,
		// TODO: add ways to customize items below
		originHeader:     "http://" + target,
		locationHeaderSH: &url.URL{Scheme: "http", Host: target},
		refererHeaderSH:  &url.URL{Scheme: "http", Host: target},
	}
	err := kc.httpRun(kish.MakeRWC(wsConn))
	if err != nil {
		log.Fatal(err)
	}
}

func (kc *KishClientHTTP) httpRun(conn io.ReadWriteCloser) error {
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
		go kc.httpForwardToTarget(clientConn, kc.target)
	}
}

func (kc *KishClientHTTP) httpForwardToTarget(clientConn io.ReadWriteCloser, target string) {
	defer clientConn.Close()
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Printf("%s\n", err)
		return
	}
	defer targetConn.Close()
	rr := &kish.HTTPRequestReader{
		Conn:        clientConn,
		ConnBufR:    bufio.NewReader(clientConn),
		BufferedReq: nil,
	}
	kish.ForwardHTTP(rr, targetConn, kc.modifyHeader, nil)
}

func replaceSHIfHasPrefix(header *http.Header, name string, prefix string, scheme string, host string) {
	valStr := header.Get(name)
	if strings.HasPrefix(valStr, prefix) {
		valURL, err := url.Parse(valStr)
		if err != nil {
			log.Printf("could not parse %s header `%s`: %v", name, valStr, err)
		} else {
			valURL.Scheme = scheme
			valURL.Host = host
			header.Set(name, valURL.String())
		}
	}
}

func (kc *KishClientHTTP) modifyHeader(req *http.Request) *http.Response {
	if kc.hostHeader != "" {
		req.Host = kc.hostHeader
	}
	if kc.locationHeaderSH != nil {
		replaceSHIfHasPrefix(&req.Header, "Location", kc.proxyURL, kc.locationHeaderSH.Scheme, kc.locationHeaderSH.Host)
	}
	if kc.originHeader != "" {
		origin := req.Header.Get("Origin")
		if strings.HasPrefix(origin, kc.proxyURL) {
			req.Header.Set("Origin", kc.originHeader)
		}
	}
	if kc.refererHeaderSH != nil {
		replaceSHIfHasPrefix(&req.Header, "Referer", kc.proxyURL, kc.refererHeaderSH.Scheme, kc.refererHeaderSH.Host)
	}
	return nil
}
