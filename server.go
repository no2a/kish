package kish

// gorilla.muxで一度つけたルートを外すことができないので、新しいrootルーターを作って差し替える
// https://github.com/gorilla/mux/issues/82

import (
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

type BuildFunc func(*mux.Router)

type KishServer struct {
	Host                string
	ProxyDomainSuffix   string
	mu                  sync.Mutex
	root                *mux.Router
	buildFuncs          map[string]BuildFunc
	TokenSet            *TokenSet
	TrustXFF            bool
	EnableTCPForwarding bool
	WebsocketHandler    string
}

func (rs *KishServer) Init() {
	rs.root = mux.NewRouter()
	rs.buildFuncs = map[string]BuildFunc{}
	rs.AddHostRouter(rs.Host, rs.configRouter)
}

func (rs *KishServer) configRouter(sr *mux.Router) {
	sr.HandleFunc("/proxy1", rs.runTcp)
	sr.HandleFunc("/proxy2", rs.runHttp)
}

func (rs *KishServer) isOccupied(host string) bool {
	_, ok := rs.buildFuncs[host]
	return ok
}

func (rs *KishServer) AddHostRouter(host string, buildFunc BuildFunc) error {
	log.Printf("register host %s", host)
	err := func() error {
		rs.mu.Lock()
		defer rs.mu.Unlock()
		if _, ok := rs.buildFuncs[host]; ok {
			return errors.New("occupied")
		}
		rs.buildFuncs[host] = buildFunc
		return nil
	}()
	if err != nil {
		return nil
	}
	sr := rs.root.Host(host).Subrouter()
	buildFunc(sr)
	return nil
}

func (rs *KishServer) DeleteHostRouter(host string) {
	log.Printf("unregister host %s", host)
	delete(rs.buildFuncs, host)
	rs.rebuild()
}

func (rs *KishServer) rebuild() {
	r := mux.NewRouter()
	for host, bf := range rs.buildFuncs {
		sr := r.Host(host).Subrouter()
		bf(sr)
	}
	rs.mu.Lock()
	rs.root = r
	rs.mu.Unlock()
}

func (rs *KishServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rs.mu.Lock()
	root := rs.root
	rs.mu.Unlock()
	root.ServeHTTP(w, r)
}
