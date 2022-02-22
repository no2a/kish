package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
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
	// なんかデフォルトだと黒背景白文字になってしまうので、元の端末の色にする
	fg := tcell.ColorDefault
	bg := tcell.ColorDefault
	tuiApp = tview.NewApplication()
	tuiText = tview.NewTextView().SetChangedFunc(func() { tuiApp.Draw() })
	tuiText.SetTextColor(fg).SetBackgroundColor(bg)
	tuiLog = tview.NewTextView().SetChangedFunc(func() { tuiApp.Draw() })
	tuiLog.SetTextColor(fg).SetBackgroundColor(bg)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(tuiText, 3, 1, false)
	flex.AddItem(tuiLog, 0, 1, false)
	tuiApp.SetRoot(flex, true)
}

func tuiRun() error {
	return tuiApp.Run()
}
