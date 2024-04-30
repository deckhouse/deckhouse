package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	FormMain              = "FormMain"
	FormSelectClusterType = "FormSelectClusterType"
)

type App struct {
	app     *tview.Application
	builder *configBuilder
	pages   *tview.Pages
}

func NewApp() *App {
	app := tview.NewApplication()
	pages := tview.NewPages()
	builder := newConfigBuilder()

	selectCls, selectFocusables := SelectClusterForm(builder, func() {
		app.Stop()
	})

	mainForm, mainFocusables := MainFormPanel(func() {
		addSwitchFocusEvent(app, pages, selectFocusables)
		pages.SwitchToPage(FormSelectClusterType)
	})

	pages.AddPage(FormMain, mainForm, true, false).
		AddPage(FormSelectClusterType, selectCls, true, false)

	addSwitchFocusEvent(app, pages, mainFocusables)
	pages.SwitchToPage(FormMain)

	return &App{
		app:     app,
		pages:   pages,
		builder: builder,
	}
}

func (a *App) Start() error {
	if err := a.app.SetRoot(a.pages, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
		return err
	}

	return nil
}

func (a *App) SetBuilder(builder *configBuilder) {}

func (a *App) Configs() []string {
	return a.builder.build()
}

func box() *tview.Box {
	return tview.NewBox()
}

func addSwitchFocusEvent(app *tview.Application, parent *tview.Pages, forFocus []tview.Primitive) {
	curIndex := 0

	parent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			curIndex = (curIndex + 1) % len(forFocus)
			app.SetFocus(forFocus[curIndex])
		case tcell.KeyEscape:
			app.Stop()
		}
		return event
	})
}
