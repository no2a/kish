package kish

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/websocket"
)

var (
	websocketUpgrader = websocket.Upgrader{}
)

type HasKeyID interface {
	GetKeyID() string
}

func (rs *KishServer) jwtUserKeyfunc(token *jwt.Token) (interface{}, error) {
	keyID := token.Claims.(HasKeyID).GetKeyID()
	key := rs.TokenSet.Get(keyID)
	if key == nil {
		return nil, errors.New("key not found")
	}
	return key, nil
}

func extractBearerToken(s string) string {
	ss := strings.Split(s, "Bearer ")
	if len(ss) != 2 {
		return ""
	}
	return strings.TrimSpace(ss[1])
}

func ReadAllAndClose(rc io.ReadCloser) []byte {
	defer rc.Close()
	bytes, _ := io.ReadAll(rc)
	return bytes
}

func Passthrough(a, b io.ReadWriter) error {
	errCh := make(chan error)
	copy := func(dest io.Writer, src io.Reader) {
		_, err := io.Copy(dest, src)
		errCh <- err
	}
	go copy(a, b)
	go copy(b, a)
	// 片方が完了するまで待つ。もう片方はこのあとCloseされる想定
	return <-errCh
}

func LogRequest(prefix string, r *http.Request, full bool) {
	if full {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		logbuf := new(bytes.Buffer)
		r.Write(logbuf)
		log.Printf("%s%s", prefix, logbuf.String())
		r.Body = io.NopCloser(bytes.NewReader(body))
	} else {
		log.Printf("%s%s %s", prefix, r.Method, r.URL.String())
	}
}

func LogResponse(prefix string, r *http.Response, full bool) {
	if full {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		logbuf := new(bytes.Buffer)
		r.Write(logbuf)
		s := logbuf.String()
		if len(s) > 4096 {
			s = s[:4096]
		}
		log.Printf("%s%s", prefix, s)
		r.Body = io.NopCloser(bytes.NewReader(body))
	} else {
		log.Printf("%s%s <- %s", prefix, r.Status, r.Request.URL.String())
	}
}

type HTTPRequestModifyFunc func(*http.Request) error

type HTTPRequestReader struct {
	Conn        io.ReadWriteCloser
	ConnBufR    *bufio.Reader
	BufferedReq *http.Request
}

func (rr *HTTPRequestReader) readRequest() (*http.Request, error) {
	req := rr.BufferedReq
	if req != nil {
		rr.BufferedReq = nil
		return req, nil
	} else {
		req, err := http.ReadRequest(rr.ConnBufR)
		return req, err
	}
}

type HTTPRequestMiddleware func(*http.Request) *http.Response
type HTTPResponseMiddleware func(*http.Response)

func ForwardHTTP(rr *HTTPRequestReader, server io.ReadWriteCloser, reqMW HTTPRequestMiddleware, respMW HTTPResponseMiddleware) error {
	// ReadResponseにbufio.Readerが必要
	serverBufR := bufio.NewReader(server)
	for {
		req, err := rr.readRequest()
		if err != nil {
			return fmt.Errorf("ReadRequest: %w", err)
		}
		LogRequest("req ", req, false)
		if reqMW != nil {
			errResp := reqMW(req)
			if errResp != nil {
				errResp.Write(rr.Conn)
				continue
			}
		}
		req.Write(server)
		res, err := http.ReadResponse(serverBufR, req)
		if err != nil {
			return fmt.Errorf("ReadResponse: %w", err)
		}
		LogResponse("res ", res, false)
		if respMW != nil {
			respMW(res)
		}
		res.Write(rr.Conn)
		if res.Close {
			return nil
		}
		if IsWebsocket(req) && res.StatusCode == 101 {
			// BufRはもう使わないので残ってるものを送り出す
			if _, err := io.CopyN(server, rr.ConnBufR, int64(rr.ConnBufR.Buffered())); err != nil {
				return fmt.Errorf("error while clearing client buffer: %w", err)
			}
			if _, err := io.CopyN(rr.Conn, serverBufR, int64(serverBufR.Buffered())); err != nil {
				return fmt.Errorf("error while clearing server buffer: %w", err)
			}
			Passthrough(rr.Conn, server)
			return nil
		}
	}
}
