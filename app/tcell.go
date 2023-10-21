package app

import (
	"arc/log"
	"runtime/debug"

	"github.com/gdamore/tcell/v2"
)

func initUi() tcell.Screen {
	screen, err := tcell.NewScreen()
	if err != nil {
		log.Debug("ERROR", err)
		panic(err)
	}

	if err := screen.Init(); err != nil {
		log.Debug("ERROR", err)
		panic(err)
	}
	return screen
}
func deinitUi(screen tcell.Screen) {
	screen.Fini()
	if err := recover(); err != nil {
		log.Debug("ERROR", "err", err)
		log.Debug("STACK", "stack", debug.Stack())
	}

	screen.EnableMouse()
}

func runUi(screen tcell.Screen, events chan tcell.Event) {
	for {
		tcEvent := screen.PollEvent()
		for {
			if ev, mouseEvent := tcEvent.(*tcell.EventMouse); !mouseEvent || ev.Buttons() != 0 {
				break
			}
			tcEvent = screen.PollEvent()
		}

		if tcEvent != nil {
			events <- tcEvent
		}
	}
}
