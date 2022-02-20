package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ClientConfig struct {
	KishURL     string `yaml:"kish-url" mapstructure:"kish-url"`
	Key         string `yaml:"key" mapstructure:"key"`
	Host        string `yaml:"hostname" mapstructure:"hostname"`
	Restriction struct {
		AllowIP   []string          `yaml:"ip" mapstructure:"ip"`
		AllowMyIP bool              `yaml:"allow-my-ip" mapstructure:"allow-my-ip"`
		Auth      map[string]string `yaml:"auth"`
	} `yaml:"restriction"`
}

var (
	flag_configFile string
	flag_enableTUI  bool
	config          ClientConfig
	rootCmd         = &cobra.Command{
		Use: "kish",
	}
	httpCmd = &cobra.Command{
		Use:   "http TARGET",
		Short: "Forward http",
		Run:   httpMain,
		Args:  httpParseArgs,
	}
	tcpCmd = &cobra.Command{
		Use:   "tcp TARGET",
		Short: "Forward tcp",
		Run:   tcpMain,
		Args:  tcpParseArgs,
	}
)

func init() {
	cobra.OnInitialize(initConfig)

	f := rootCmd.PersistentFlags()
	f.StringVar(&flag_configFile, "config", "", "config file (default is $HOME/.kish.yaml)")
	f.BoolVar(&flag_enableTUI, "enable-tui", false, "enable UI")
	f.StringP("hostname", "", "", "assign fixed domain name instead of random one")
	f.BoolP("allow-my-ip", "", false, "automatically add global IP of this machine to allow-ip")
	viper.BindPFlags(f)

	rootCmd.AddCommand(httpCmd)
	f = httpCmd.Flags()
	f.StringVar(&flag_hostHeader, "host-header", "", "value of Host header of forawarded requests")

	rootCmd.AddCommand(tcpCmd)
	f = tcpCmd.Flags()
}

func initConfig() {
	if flag_configFile != "" {
		viper.SetConfigFile(flag_configFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".kish")
	}
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if flag_configFile != "" {
				log.Fatalf("specified config file not found: %s", err)
			} else {
				// ok, ignore
			}
		} else {
			log.Fatalf("error reading config file: %s", err)
		}
	} else {
		viper.Unmarshal(&config)
	}
}

func findAndParseFlags() (*cobra.Command, error) {
	cmd, args, err := rootCmd.Find(os.Args[1:])
	if err != nil {
		return nil, err
	}
	err = cmd.ParseFlags(args)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func main() {
	cmd, err := findAndParseFlags()
	// FIXME: positional argsのチェックがtui起動後になる
	if err == nil {
		if flag_enableTUI {
			tuiInit()
			log.SetOutput(tuiLog)
			// go tunRun()にするとctrl-Cを2回押さないと終了しなかったのでcmdのほうをgoする
			go cmd.Execute()
			err = tuiRun()
		} else {
			err = cmd.Execute()
		}
	}
	if err != nil {
		os.Exit(1)
	}
}
