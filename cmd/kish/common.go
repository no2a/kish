package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/no2a/kish"
)

func parseKey(key string) (string, string) {
	parts := strings.SplitN(key, "/", 2)
	return parts[0], parts[1]
}

func canonicalizeTargetArg(target string) string {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host = ""
		port = target
	}
	if port == "" {
		return ""
	}
	num, err := strconv.Atoi(port)
	if err == nil && 1 <= num && num <= 65535 {
		// ok
	} else {
		return ""
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func dialKish(pathAppend string) (*websocket.Conn, string, http.Header) {
	wsURL, err := url.Parse(config.KishURL)
	if err != nil {
		log.Fatalf("kish-url `%s` is invalid: %s", config.KishURL, err)
	}
	wsURL.Path = path.Join(wsURL.Path, pathAppend)

	keyID, keySecret := parseKey(config.Key)
	if keyID == "" || keySecret == "" {
		log.Fatalf("key is invalid")
	}

	params := kish.ProxyParameters{
		Host:      config.Host,
		AllowIP:   config.Restriction.AllowIP,
		AllowMyIP: config.Restriction.AllowMyIP,
		BasicAuth: config.Restriction.Auth,
	}
	token, err := kish.GenerateToken(time.Now(), &params, []byte(keySecret), keyID)
	if err != nil {
		log.Fatal(err)
	}
	origin := mapWsToHttp(wsURL.Scheme) + "://" + wsURL.Host
	wsConn, proxyURL, header, err := makeWsConn(wsURL.String(), origin, token)
	if err != nil {
		log.Fatal(err)
	}
	return wsConn, proxyURL, header
}

func makeWsConn(wsUrl string, origin string, token string) (*websocket.Conn, string, http.Header, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	header.Set("Origin", origin)
	conn, resp, err := websocket.DefaultDialer.Dial(wsUrl, header)
	if err != nil {
		if resp != nil {
			msg := resp.Header.Get("X-Error-Message")
			if msg != "" {
				msg = fmt.Sprintf("%s: %s", resp.Status, msg)
			} else {
				msg = resp.Status
			}
			err = fmt.Errorf("%w: %s", err, msg)
		}
		return nil, "", nil, err
	}
	proxyURL := resp.Header.Get("X-Kish-URL")
	return conn, proxyURL, resp.Header, nil
}

func mapWsToHttp(scheme string) string {
	if scheme == "wss" {
		return "https"
	}
	return "http"
}
