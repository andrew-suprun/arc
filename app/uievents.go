package app

import (
	"arc/log"
	"path/filepath"
	"time"

	"os/exec"

	"github.com/gdamore/tcell/v2"
)

func newUiEvents() chan tcell.Event {
	return make(chan tcell.Event)
}

func (app *appState) handleUiEvent(event tcell.Event) {
	switch event := event.(type) {
	case *tcell.EventKey:
		app.handleKeyEvent(event)

	case *tcell.EventMouse:
		app.handleMouseEvent(event)

	case *tcell.EventResize:
		app.sync = true
		app.screenWidth, app.screenHeight = event.Size()
	}
}

func (app *appState) handleKeyEvent(event *tcell.EventKey) {
	log.Debug("handleKeyEvent", "key", event.Name())
	switch event.Name() {
	case "Up":
		app.curArchive.curFolder.selectedIdx--
		app.makeSelectedVisible = true

	case "Down":
		app.curArchive.curFolder.selectedIdx++
		app.makeSelectedVisible = true

	case "PgUp":
		app.curArchive.curFolder.selectedIdx -= app.screenHeight - 4
		app.curArchive.curFolder.offsetIdx -= app.screenHeight - 4

	case "PgDn":
		app.curArchive.curFolder.selectedIdx += app.screenHeight - 4
		app.curArchive.curFolder.offsetIdx += app.screenHeight - 4

	case "Home":
		app.curArchive.curFolder.selectedIdx = 0
		app.makeSelectedVisible = true

	case "End":
		folder := app.curArchive.curFolder
		folder.selectedIdx = len(folder.children) - 1
		app.makeSelectedVisible = true

	case "Right":
		folder := app.curArchive.curFolder
		child := folder.children[folder.selectedIdx]
		if child.folder != nil {
			app.curArchive.curFolder = child
		}

	case "Left":
		parent := app.curArchive.curFolder.parent
		if parent != nil {
			app.curArchive.curFolder = parent
		}

	case "Ctrl+F":
		archive := app.curArchive
		folder := archive.curFolder
		name := folder.children[folder.selectedIdx].name
		path := filepath.Join(archive.rootPath, filepath.Join(folder.fullPath()...), name)
		exec.Command("open", "-R", path).Start()

	case "Enter":
		archive := app.curArchive
		folder := archive.curFolder
		name := folder.children[folder.selectedIdx].name
		path := filepath.Join(archive.rootPath, filepath.Join(folder.fullPath()...), name)
		exec.Command("open", path).Start()

	case "Ctrl+C":
		app.fs.Quit()

	case "Ctrl+R":
		// TODO Resole

	case "Ctrl+A":
		// TODO Resole All

	case "Tab":
		(&tabState{app: app}).tab()

	case "Backspace2": // Ctrl+Delete
		// TODO Delete

	case "F10":
		// TODO Switch Debug On/Off

	case "F12":
		// TODO Print App State

	default:
		if event.Name() >= "Rune[1]" && event.Name() <= "Rune[9]" {
			arcIdx := int(event.Name()[5] - '1')
			if arcIdx < len(app.archives) {
				app.curArchive = app.archives[arcIdx]
			}
		}
	}
}

func (app *appState) handleMouseEvent(event *tcell.EventMouse) {
	x, y := event.Position()
	if event.Buttons() == 256 || event.Buttons() == 512 {
		if y >= 3 && y < app.screenHeight-1 {
			folder := app.curArchive.curFolder
			if event.Buttons() == 512 {
				folder.offsetIdx++
			} else {
				folder.offsetIdx--
			}
		}
		return
	}

	if y == 1 {
		for _, target := range app.folderTargets {
			if target.offset <= x && target.offset+target.width > x {
				app.curArchive.curFolder = app.curArchive.getFolder(target.path)
				return
			}
		}
	} else if y == 2 {
		for i, target := range app.sortTargets {
			if target.offset <= x && x < target.offset+target.width {
				folder := app.curArchive.curFolder
				if folder.sortColumn == target.sortColumn {
					folder.sortAscending[i] = !folder.sortAscending[i]
				} else {
					folder.sortColumn = target.sortColumn
				}
				folder.sorted = false
			}
		}
	} else if y >= 3 && y < app.screenHeight-1 {
		folder := app.curArchive.curFolder
		curSelectedIdx := folder.selectedIdx
		idx := folder.offsetIdx + y - 3
		if idx < len(folder.children) {
			folder.selectedIdx = folder.offsetIdx + y - 3
		}
		if curSelectedIdx == folder.selectedIdx && time.Since(app.lastClickTime).Milliseconds() < 500 {
			entry := folder.children[curSelectedIdx]
			if entry.children != nil {
				path := append(entry.path(), entry.name)
				app.curArchive.curFolder = app.curArchive.getFolder(path)
			}
		}
		app.lastClickTime = time.Now()
	}
}

type tabState struct {
	app           *appState
	curArchive    *archive
	curFile       *file
	firstArchive  *archive
	firstFolder   *file
	firstFileIdx  int
	foundSameFile bool
	done          bool
}

func (ts *tabState) tab() {
	folder := ts.app.curArchive.curFolder
	ts.curFile = folder.children[folder.selectedIdx]
	if ts.curFile.folder != nil {
		return
	}
	for _, ts.curArchive = range ts.app.archives {
		ts.walkFiles(ts.curArchive.rootFolder)
	}
	if !ts.done {
		ts.app.curArchive = ts.firstArchive
		ts.curArchive.curFolder = ts.firstFolder
		folder.selectedIdx = ts.firstFileIdx
		ts.app.makeSelectedVisible = true
	}
}

func (ts *tabState) walkFiles(folder *file) {
	for idx, child := range folder.children {
		if ts.done {
			return
		}
		if child.folder != nil {
			ts.walkFiles(child)
		} else if ts.curFile.hash == child.hash {
			if ts.foundSameFile {
				ts.app.curArchive = ts.curArchive
				ts.curArchive.curFolder = folder
				folder.selectedIdx = idx
				ts.app.makeSelectedVisible = true
				ts.done = true
				return
			}
			if ts.firstArchive == nil {
				ts.firstArchive = ts.curArchive
				ts.firstFolder = folder
				ts.firstFileIdx = idx
			}
			if child == ts.curFile {
				ts.foundSameFile = true
			}
		}
	}
}
