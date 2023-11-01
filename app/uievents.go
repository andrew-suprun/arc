package app

import (
	"arc/log"
	"path/filepath"
	"slices"
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
		folder := app.curArchive.curFolder
		app.resolve(app.curArciveIdx(), folder.children[folder.selectedIdx], true)

	case "Ctrl+A":
		folder := app.curArchive.curFolder
		for _, child := range folder.children {
			app.resolve(app.curArciveIdx(), child, false)
		}

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
		ts.curArchive.rootFolder.walk(ts.handle)
	}
	if !ts.done {
		ts.app.curArchive = ts.firstArchive
		ts.curArchive.curFolder = ts.firstFolder
		folder.selectedIdx = ts.firstFileIdx
		ts.app.makeSelectedVisible = true
	}
}

func (ts *tabState) handle(idx int, f *file) bool {
	if ts.curFile.hash == f.hash {
		if ts.foundSameFile {
			ts.app.curArchive = ts.curArchive
			ts.curArchive.curFolder = f
			f.selectedIdx = idx
			ts.app.makeSelectedVisible = true
			return false
		}
		if ts.firstArchive == nil {
			ts.firstArchive = ts.curArchive
			ts.firstFolder = f
			ts.firstFileIdx = idx
		}
		if f == ts.curFile {
			ts.foundSameFile = true
		}
	}
	return true
}

func (app *appState) resolve(sourceArcIdx int, source *file, explicit bool) {
	if source.state != divergent {
		return
	}
	if source.folder != nil {
		for _, child := range source.children {
			app.resolve(sourceArcIdx, child, false)
		}
		return
	}
	sameHash := make([][]*file, len(app.archives))
	for arcIdx, archive := range app.archives {
		archive.rootFolder.walk(func(_ int, child *file) bool {
			if child.hash == source.hash {
				sameHash[arcIdx] = append(sameHash[arcIdx], child)
			}
			return true
		})
	}

	if !explicit && len(sameHash[sourceArcIdx]) > 1 {
		return
	}

	// copy missing
	roots := []string{}
	for arcIdx, archive := range app.archives {
		if len(sameHash[arcIdx]) == 0 {
			roots = append(roots, archive.rootPath)
		}
	}
	if len(roots) > 0 {
		app.fs.Copy(filepath.Join(source.fullPath()...), app.curArchive.rootPath, roots...)
	}

	// rename/delete as necessary
	for arcIdx, files := range sameHash {
		archive := app.archives[arcIdx]
		if len(files) == 0 {
			continue
		}
		keep := files[0]
		for _, file := range files {
			if slices.Equal(file.fullPath(), source.fullPath()) {
				keep = source
				break
			}
		}
		for _, file := range files {
			if file == keep {
				if !slices.Equal(file.fullPath(), source.fullPath()) {
					app.fs.Rename(archive.rootPath, filepath.Join(file.fullPath()...), filepath.Join(source.fullPath()...))
				}
			} else {
				app.fs.Delete(filepath.Join(archive.rootPath, filepath.Join(file.fullPath()...)))
			}
		}
	}
}
