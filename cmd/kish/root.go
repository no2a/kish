package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	_, fMain := parseArgs()
	err := func() error {
		configPath, err := configPath(*flag_configFile)
		if err != nil {
			return err
		}
		err = initConfig(configPath)
		if err != nil {
			return err
		}
		return nil
	}()
	if err == nil {
		if *flag_enableTUI {
			tuiInit()
			log.SetOutput(tuiLog)
			// go tunRun()にするとctrl-Cを2回押さないと終了しなかったのでcmdのほうをgoする
			go fMain()
			err = tuiRun()
		} else {
			fMain()
		}
	}
	if err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
