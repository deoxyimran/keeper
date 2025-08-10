package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/op"
	"gioui.org/unit"
	myapp "github.com/deoxyimran/keeper/app"
)

func main() {
	w, h := 900, 600
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("Keeper"),
			app.MinSize(unit.Dp(w), unit.Dp(h)),
			app.MaxSize(unit.Dp(w), unit.Dp(h)),
		)
		err := run(window)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(window *app.Window) error {
	// Init app
	a := myapp.NewApp()
	// Load saved notes
	a.Load()
	// Run loop
	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			// Save notes
			a.Save()
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
