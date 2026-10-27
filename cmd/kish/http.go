package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/hashicorp/yamux"
	"github.com/no2a/kish"
)

type KishClientHTTP struct {
	proxyURL         *url.URL
	target           string
	hostHeader       string
	originHeaderSH   *url.URL
	locationHeaderSH *url.URL
	refererHeaderSH  *url.URL
}

func httpMain() {
	defaultPort := 80
	kish.LogHttpRequestDetail = *flag_httpLogRequestDetail
	kish.LogHttpResponseDetail = *flag_httpLogRequestDetail
	target := canonicalizeTargetArg(*flag_httpTarget, -1)
	targetOmitDefaultPort := canonicalizeTargetArg(*flag_httpTarget, defaultPort)
	wsConn, proxyURL, header := dialKish("proxy2")
	tuiWriteText(fmt.Sprintf("%s -> %s\n", proxyURL, target))
	tuiWriteText(fmt.Sprintf("Host Header -> %s\n", *flag_hostHeader))
	tuiWriteText(fmt.Sprintf("Allow IP: %s\n", header.Get("X-Kish-Allow-IP")))

	// Scheme in the header values are always https, as kish-server sends "X-Forwarded-Proto: https"
	scheme := "https"

	kc := KishClientHTTP{
		proxyURL:   proxyURL,
		target:     target,
		hostHeader: *flag_hostHeader,
		// TODO: add ways to customize items below
		originHeaderSH:   &url.URL{Scheme: scheme, Host: targetOmitDefaultPort},
		locationHeaderSH: &url.URL{Scheme: scheme, Host: targetOmitDefaultPort},
	}
	if *flag_modifyReferer {
		kc.refererHeaderSH = &url.URL{Scheme: scheme, Host: targetOmitDefaultPort}
	}
	tuiWriteText(fmt.Sprintf("Origin Header -> %s\n", kc.originHeaderSH.String()))
	tuiWriteText(fmt.Sprintf("Location Header -> %s\n", kc.locationHeaderSH.String()))
	if kc.refererHeaderSH != nil {
		tuiWriteText(fmt.Sprintf("Referer Header -> %s\n", kc.refererHeaderSH.String()))
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
	kish.ForwardHTTP(rr, targetConn, kc.modifyRequestHeader, kc.modifyResponseHeader)
}

func replaceSHIfHasPrefix(header *http.Header, name string, from *url.URL, to *url.URL) {
	valStr := header.Get(name)
	valURL, err := url.Parse(valStr)
	if err != nil {
		log.Printf("could not parse %s header as a url: %v", name, err)
	} else if valURL.Scheme == from.Scheme && valURL.Host == from.Host {
		valURL.Scheme = to.Scheme
		valURL.Host = to.Host
		newValStr := valURL.String()
		header.Set(name, newValStr)
		log.Printf("set %s header to %s from %s", name, newValStr, valStr)
	}
}

func (kc *KishClientHTTP) modifyRequestHeader(req *http.Request) *http.Response {
	if kc.hostHeader != "" {
		req.Host = kc.hostHeader
		log.Printf("set %s header to %s from %s", "Host", kc.hostHeader, req.Host)
	}
	if kc.originHeaderSH != nil {
		replaceSHIfHasPrefix(&req.Header, "Origin", kc.proxyURL, kc.originHeaderSH)
	}
	if kc.refererHeaderSH != nil {
		replaceSHIfHasPrefix(&req.Header, "Referer", kc.proxyURL, kc.refererHeaderSH)
	}
	return nil
}

func (kc *KishClientHTTP) modifyResponseHeader(resp *http.Response) {
	if kc.locationHeaderSH != nil {
		replaceSHIfHasPrefix(&resp.Header, "Location", kc.locationHeaderSH, kc.proxyURL)
	}
}
