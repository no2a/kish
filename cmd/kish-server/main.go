package main

import (
	"log"
	"net/http"
	"os"

	"github.com/no2a/kish"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

type ServerConfig struct {
	Host                string `yaml:"host"`
	DomainSuffix        string `yaml:"domain-suffix"`
	ListenAddr          string `yaml:"listen"`
	TrustXFF            bool   `yaml:"trust-x-forwarded-for"`
	TokenSetPath        string `yaml:"account"`
	TLSCert             string `yaml:"tls-cert"`
	TLSKey              string `yaml:"tls-key"`
	EnableTCPForwarding bool   `yaml:"enable-tcp-forwarding"`
	WebsocketHandler    string `yaml:"websocket-handler"`
}

var (
	config ServerConfig
)

func parse() string {
	p := kingpin.Flag("config", "config file").Required().ExistingFile()
	kingpin.Parse()
	return *p
}

func loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(f)
	err = dec.Decode(&config)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	configFile := parse()
	err := loadConfig(configFile)
	if err != nil {
		panic(err)
	}
	serverMain()
}

func serverMain() {
	log.Printf("config dump: %#v", config)
	rs := &kish.KishServer{
		Host:                config.Host,
		ProxyDomainSuffix:   config.DomainSuffix,
		TokenSet:            &kish.TokenSet{Path: config.TokenSetPath},
		TrustXFF:            config.TrustXFF,
		EnableTCPForwarding: config.EnableTCPForwarding,
		WebsocketHandler:    config.WebsocketHandler,
	}
	rs.Init()
	var err error
	if config.TLSCert != "" {
		err = http.ListenAndServeTLS(config.ListenAddr, config.TLSCert, config.TLSKey, rs)
	} else {
		err = http.ListenAndServe(config.ListenAddr, rs)
	}
	if err != nil {
		panic(err)
	}
}
