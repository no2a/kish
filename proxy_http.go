package kish

import (
	"bufio"
	"context"
	"crypto/rand"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

type proxy2Struct struct {
	host      string
	ipset     IPSet
	trustXFF  bool
	basicAuth map[string]string

	session *yamux.Session

	// websocket or direct : どの方式がいいか未決なので設定で選べるようにする。デフォルト("")では中継せずBadRequestを返す
	websocketHandler string
}

func makeRandomStr(length int) (string, error) {
	const alnum = "abcdefghijklmnopqrstuvwxyz0123456789"
	var result string
	nums := make([]byte, length)
	_, err := rand.Read(nums)
	if err != nil {
		return "", err
	}
	for _, n := range nums {
		i := int(n) % len(alnum)
		result += string(alnum[i])
	}
	return result, nil
}

func makeNgrokishDomainName(remoteIP string, suffix string) (string, error) {
	rnd, err := makeRandomStr(4)
	if err != nil {
		return "", err
	}
	host := strings.Replace(remoteIP, ".", "-", -1)
	host = strings.Replace(host, ":", "-", -1)
	return rnd + "-" + host + suffix, nil
}

func (rs *KishServer) runHttp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	t := extractBearerToken(r.Header.Get("Authorization"))
	claims, err := validateToken(t, rs.TokenSet)
	if claims == nil || err != nil {
		log.Printf("validateToken failed: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
		cancel()
		return
	}

	proxy2 := proxy2Struct{
		trustXFF:         rs.TrustXFF,
		websocketHandler: rs.WebsocketHandler,
	}

	proxy2.basicAuth = map[string]string{}
	for user, pass := range claims.BasicAuth {
		proxy2.basicAuth[user] = pass
	}

	remoteIP := GetRemoteIP(r, rs.TrustXFF)

	if claims.Host == "" {
		ngorkishDN, err := makeNgrokishDomainName(remoteIP, rs.ProxyDomainSuffix)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		proxy2.host = ngorkishDN
	} else {
		if strings.Index(claims.Host, ".") == -1 {
			claims.Host += rs.ProxyDomainSuffix
		}
		if !strings.HasSuffix(claims.Host, rs.ProxyDomainSuffix) {
			w.Header().Set("X-Error-Message", "wrong domain name")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// 使えない文字が入ってないかチェック
		dc := strings.TrimSuffix(claims.Host, rs.ProxyDomainSuffix)
		if matched, _ := regexp.MatchString("^[a-z0-9][-a-z0-9]*$", strings.ToLower(dc)); !matched {
			w.Header().Set("X-Error-Message", "wrong domain name")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		proxy2.host = claims.Host
	}
	// hostが既に使われていないかチェック
	// ランダム生成の場合はやり直せるがめんどうなのでそのままエラーにしている
	if rs.isOccupied(proxy2.host) {
		log.Printf("host %s is occupied", proxy2.host)
		w.Header().Set("X-Error-Message", "domain name is already in use")
		w.WriteHeader(http.StatusConflict)
		return
	}

	for _, cidr := range claims.AllowIP {
		proxy2.ipset.Add(cidr)
	}
	if claims.AllowMyIP {
		if strings.Index(remoteIP, ":") != -1 {
			// 多分IPv6
			proxy2.ipset.Add(remoteIP + "/128")
		} else {
			proxy2.ipset.Add(remoteIP + "/32")
		}
	}

	respHeader := http.Header{}
	respHeader.Set("X-Kish-URL", "https://"+proxy2.host)
	respHeader.Set("X-Kish-Allow-IP", proxy2.ipset.String())
	c, err := websocketUpgrader.Upgrade(w, r, respHeader)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	proxy2.session, err = yamux.Server(MakeRWC(c), nil)
	if err != nil {
		log.Printf("yamux.Server: %s", err)
		return
	}
	go func() {
		<-proxy2.session.CloseChan()
		cancel()
	}()
	defer proxy2.session.Close()

	// isOccupiedでチェックされてからここに至る間に同じhostが取得されている可能性がある。
	// レスポンスはupgradeで既に送出済みなのでエラーを返すことはできず接続を切るしかない。
	// レアケースなのでそれでよしとする。対処するとしたら、予約してから確定という方式は可能そう
	err = rs.AddHostRouter(proxy2.host, func(sr *mux.Router) { sr.PathPrefix("/").HandlerFunc(proxy2.normalHandler) })
	if err != nil {
		log.Printf("AddHostRouter: %s", err)
		return
	}
	defer rs.DeleteHostRouter(proxy2.host)
	log.Printf("tunnel for %s has been established", proxy2.host)
	<-ctx.Done()
}

func GetRemoteIP(req *http.Request, trustXFF bool) string {
	if trustXFF {
		xff := req.Header.Get("X-Forwarded-For")
		if xff != "" {
			return xff
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return ""
	}
	return host
}

func (p *proxy2Struct) checkRemoteIP(req *http.Request) (string, bool) {
	remoteIP := GetRemoteIP(req, p.trustXFF)
	return remoteIP, p.ipset.ContainsIPString(remoteIP)
}

func (p *proxy2Struct) checkAuth(req *http.Request) bool {
	if len(p.basicAuth) == 0 {
		return true
	}
	usernameIn, passwordIn, ok := req.BasicAuth()
	if !ok {
		return false
	}
	password, exist := p.basicAuth[usernameIn]
	if !exist {
		return false
	}
	return password == passwordIn
}

func (p *proxy2Struct) normalHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("new connection to %s", p.host)
	log.Printf("Host: %s", req.Host)
	w.Header().Set("X-Robots-Tag", "none")
	remoteIP, okIP := p.checkRemoteIP(req)
	if !okIP {
		http.Error(w, "Access form your IP is not allowed", http.StatusForbidden)
		return
	}
	okAuth := p.checkAuth(req)
	if !okAuth {
		w.Header().Set("WWW-Authenticate", "Basic")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	serverConn, err := p.session.Open()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer serverConn.Close()
	req.Header.Set("X-Forwarded-For", remoteIP)
	req.Header.Set("X-Forwarded-Proto", "https")

	if p.websocketHandler == "websocket" {
		// websocket.Connを両サイドに作ってそれらをつなぐ方式
		if IsWebsocket(req) {
			dc := func(ctx context.Context, network, addr string) (net.Conn, error) {
				return p.session.Open()
			}
			wsd := &websocket.Dialer{NetDialContext: dc}
			wsresp, err := PassthroughWebsocket(wsd, w, req)
			// ここに来た時点でコネクションはhijackされてClose済み。後始末は不要。
			if err != nil {
				log.Printf("error PasthroughWebsocket: %s", err)
				log.Printf("  resp headers: %#v", wsresp.Header)
				// wsrespはwへの送出に失敗ずみなので捨てるしかない
			} else {
				// wsrespはすでにwに送出されているのでなにもする必要はない
			}
			return
		}
	}

	err = req.Write(serverConn)
	if err != nil {
		log.Printf("error req.Write: %s", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	serverBufR := bufio.NewReader(serverConn)
	resp, err := http.ReadResponse(serverBufR, req)
	if err != nil {
		log.Printf("error ReadResponse: %s", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if p.websocketHandler == "" {
		// websocketを中継しない
		if IsWebsocket(req) && resp.StatusCode == 101 {
			http.Error(w, "forwarding websocket is disabled", http.StatusBadRequest)
			return
		}
	}

	_, err = writeResponse(resp, w)
	if err != nil {
		log.Printf("responseToResponseWriter: %s", err)
		return
	}
	log.Printf("resp Connection:%s", resp.Header.Get("Connection"))

	if p.websocketHandler == "direct" {
		// websocketを作らずhijackしたnet.Connとyamuxストリームを直接つなぐ方式
		if IsWebsocket(req) && resp.StatusCode == 101 {
			hijackToWebsocket(w, req, serverConn)
		}
	}
	return
}

func writeResponse(r *http.Response, w http.ResponseWriter) (int64, error) {
	defer r.Body.Close()
	for k, v := range r.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(r.StatusCode)
	return io.Copy(w, r.Body)
}

func IsWebsocket(req *http.Request) bool {
	r := strings.EqualFold(req.Header.Get("Connection"), "upgrade")
	r = r && strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
	return r
}

func hijackToWebsocket(w http.ResponseWriter, req *http.Request, serverConn io.ReadWriteCloser) {
	defer serverConn.Close()
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, bufRW, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	if bufRW.Writer.Buffered() > 0 {
		log.Printf("bufRW unexpectedly has buffered data")
		// TODO: return HTTP response
		return
	}
	log.Printf("start websocket passthrough")
	Passthrough(conn, serverConn)
}

func GetWsURL(req *http.Request) *url.URL {
	// req.URLがパス部分以降しか含まれていない場合(常に?)、補完する
	wsURL := *req.URL
	if wsURL.Scheme == "https" {
		wsURL.Scheme = "wss"
	} else {
		wsURL.Scheme = "ws"
	}
	if wsURL.Host == "" {
		wsURL.Host = req.Host
	}
	return &wsURL
}

func PassthroughWebsocket(dialer *websocket.Dialer, w http.ResponseWriter, req *http.Request) (*http.Response, error) {
	wsURL := GetWsURL(req)
	wsHeader := http.Header{}
	for k, v := range req.Header {
		wsHeader[k] = v
	}
	// Avoid error of duplicate header not allowed
	wsHeader.Del("Connection")
	wsHeader.Del("Upgrade")
	wsHeader.Del("Sec-WebSocket-Extensions")
	wsHeader.Del("Sec-Websocket-Key")
	wsHeader.Del("Sec-WebSocket-Protocol")
	wsHeader.Del("Sec-Websocket-Version")

	wsDest, res, err := dialer.Dial(wsURL.String(), wsHeader)
	if err != nil {
		_, err2 := writeResponse(res, w)
		log.Printf("responseToResponseWriter: %s", err2)
		// 返すのは元のerr。err2のハンドリングはログに出すだけ
		return res, err
	}
	wsSrc, err := websocketUpgrader.Upgrade(w, req, nil)
	go func() {
		defer wsDest.Close()
		defer wsSrc.Close()
		Passthrough(MakeRWC(wsSrc), MakeRWC(wsDest))
	}()
	return res, nil
}
