package main

import (
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"gopkg.in/yaml.v3"
)

type ClientConfig struct {
	KishURL     string `yaml:"kish-url"`
	Key         string `yaml:"key"`
	Host        string `yaml:"hostname"`
	Restriction struct {
		AllowIP   []string          `yaml:"ip"`
		AllowMyIP bool              `yaml:"allow-my-ip"`
		Auth      map[string]string `yaml:"auth"`
	} `yaml:"restriction"`
}

var (
	flag_configFile       *string
	flag_enableTUI        *bool
	flag_hostname         *string
	flag_hostname_passed  bool
	flag_allowMyIP        *bool
	flag_allowMyIP_passed bool

	flag_httpTarget *string
	flag_hostHeader *string
	flag_modifyReferer *bool

	flag_tcpTarget *string

	config ClientConfig
)

// フラグが渡されなかったときconfigの値を採用するためのしくみ
// フラグが渡されるとtargetがtrueになる
func setPassed(target *bool) kingpin.Action {
	*target = false
	f := func(pc *kingpin.ParseContext) error {
		*target = true
		return nil
	}
	return f
}

func parseArgs() (string, func()) {
	app := kingpin.New("", "")

	flag_configFile = app.Flag("config", "config file (default to .kish in the home directory)").String()
	flag_enableTUI = app.Flag("enable-tui", "enable UI").Bool()
	flag_hostname = app.Flag("hostname", "assign fixed domain name instead of random one").
		Action(setPassed(&flag_hostname_passed)).String()
	flag_allowMyIP = app.Flag("allow-my-ip", "automatically add global IP of this machine to allow-ip").
		Action(setPassed(&flag_allowMyIP_passed)).Bool()

	http := app.Command("http", "")
	flag_hostHeader = http.Flag("host-header", "value of Host header of forawarded requests").String()
	flag_modifyReferer = http.Flag("modify-referer", "replace scheme and host part of incoming referer header").Bool()
	flag_httpTarget = http.Arg("target", "").Required().String()

	tcp := app.Command("tcp", "")
	flag_tcpTarget = tcp.Arg("target", "").Required().String()

	commandMain := map[string]func(){
		"http": httpMain,
		"tcp":  tcpMain,
	}

	command, err := app.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}
	return command, commandMain[command]
}

func configPath(s string) (string, error) {
	if s != "" {
		return s, nil
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", nil
	}
	return filepath.Join(homedir, ".kish"), nil
}

func initConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(f)
	err = d.Decode(&config)
	if err != nil {
		return err
	}
	if flag_allowMyIP_passed {
		config.Restriction.AllowMyIP = *flag_allowMyIP
	}
	if flag_hostname_passed {
		config.Host = *flag_hostname
	}
	return nil
}
