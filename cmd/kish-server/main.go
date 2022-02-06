package main

import (
	"log"
	"net/http"
	"os"

	"github.com/no2a/kish"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ServerConfig struct {
	Host                string `mapstructure:"host"`
	DomainSuffix        string `mapstructure:"domain-suffix"`
	ListenAddr          string `mapstructure:"listen"`
	TrustXFF            bool   `mapstructure:"trust-x-forwarded-for"`
	TokenSetPath        string `mapstructure:"account"`
	TLSCert             string `mapstructure:"tls-cert"`
	TLSKey              string `mapstructure:"tls-key"`
	EnableTCPForwarding bool   `mapstructure:"enable-tcp-forwarding"`
	WebsocketHandler    string `mapstructure:"websocket-handler"`
}

var (
	configFile string
	config     ServerConfig
	rootCmd    = &cobra.Command{
		Run: serverMain,
	}
)

func initConfig() {
	viper.SetConfigFile(configFile)
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("specified config file not found: %s", err)
		} else {
			log.Fatalf("error reading config file: %s", err)
		}
	} else {
		viper.Unmarshal(&config)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	f := rootCmd.PersistentFlags()
	f.StringVar(&configFile, "config", "kish-server-config.yaml", "config file")
}

func serverMain(cmd *cobra.Command, args []string) {
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

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
