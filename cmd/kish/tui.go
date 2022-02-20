package main

import (
	"fmt"

	"github.com/rivo/tview"
)

var (
	tuiApp  *tview.Application
	tuiText *tview.TextView
	tuiLog  *tview.TextView
)

func tuiWriteText(text string) {
	if tuiText != nil {
		fmt.Fprint(tuiText, text)
	} else {
		fmt.Print(text)
	}
}

func tuiInit() {
	tuiApp = tview.NewApplication()
	tuiText = tview.NewTextView().SetChangedFunc(func() { tuiApp.Draw() })
	tuiLog = tview.NewTextView().SetChangedFunc(func() { tuiApp.Draw() })
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(tuiText, 3, 1, false)
	flex.AddItem(tuiLog, 0, 1, false)
	tuiApp.SetRoot(flex, true)
}

func tuiRun() error {
	return tuiApp.Run()
}
