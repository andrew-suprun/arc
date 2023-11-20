package app

import (
	"arc/log"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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
		app.resolve(app.curArciveIdx(), folder.children[folder.selectedIdx])

	case "Ctrl+A":
		app.resolve(app.curArciveIdx(), app.curArchive.curFolder)

	case "Tab":
		(&tabState{app: app}).tab()

	case "Backtab":
		// TODO

	case "Backspace2": // Ctrl+Delete
		folder := app.curArchive.curFolder
		app.delete(app.curArciveIdx(), folder.children[folder.selectedIdx])

	case "F10":
		// TODO Switch Debug On/Off

	case "F12":
		log.Debug("----")

		for i, file := range app.curArchive.curFolder.children {
			log.Debug("line", "idx", i, "file", file)
		}

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
	xx, y := event.Position()
	x := width(xx)
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
				app.curArchive.curFolder = app.curArchive.findFile(target.path)
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
		if app.lastX == x && app.lastY == y && time.Since(app.lastClickTime).Milliseconds() < 500 {
			entry := folder.children[curSelectedIdx]
			if entry.folder != nil {
				path := append(entry.path(), entry.name)
				app.curArchive.curFolder = app.curArchive.findFile(path)
			}
		}
		app.lastClickTime = time.Now()
		app.lastX = x
		app.lastY = y
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
		if ts.curArchive.rootFolder.walk(ts.handle) == stop {
			break
		}
	}
	if !ts.done {
		ts.app.curArchive = ts.firstArchive
		ts.app.curArchive.curFolder = ts.firstFolder
		ts.app.curArchive.curFolder.selectedIdx = ts.firstFileIdx
		ts.app.makeSelectedVisible = true
	}
}

func (ts *tabState) handle(idx int, f *file) handleResult {
	if ts.curFile.hash == f.hash {
		if ts.foundSameFile {
			ts.app.curArchive = ts.curArchive
			ts.app.curArchive.curFolder = f.parent
			f.parent.selectedIdx = idx
			ts.app.makeSelectedVisible = true
			ts.done = true
			return stop
		}
		if ts.firstArchive == nil {
			ts.firstArchive = ts.curArchive
			ts.firstFolder = f.parent
			ts.firstFileIdx = idx
		}
		if f == ts.curFile {
			ts.foundSameFile = true
		}
	}
	return advance
}

func (app *appState) resolve(sourceArcIdx int, source *file) {
	if source.state != divergent {
		return
	}
	if source.folder != nil {
		for _, child := range source.children {
			app.resolve(sourceArcIdx, child)
		}
		return
	}

	path := source.fullPath()
	roots := make([]string, 0, len(app.archives))
	for _, archive := range app.archives {
		otherFile := archive.findFile(path)
		if otherFile != nil && otherFile.hash == source.hash {
			continue
		}
		app.clearPath(archive, source.fullPath())

		renamed := false
		archive.rootFolder.walk(func(_ int, child *file) handleResult {
			if child.hash == source.hash && child.state == divergent {
				archive.deleteFile(child)
				clone := source.clone()
				clone.state = hashed
				folder := archive.getFile(source.path())
				folder.children = append(folder.children, clone)
				clone.parent = folder
				folder.sorted = false
				app.fs.Rename(archive.rootPath, filepath.Join(child.fullPath()...), filepath.Join(clone.fullPath()...))
				renamed = true
				return stop
			}
			return advance
		})

		if !renamed {
			roots = append(roots, archive.rootPath)
		}
	}

	if len(roots) > 0 {
		source.state = pending
		for _, newRoot := range roots {
			archive := app.archive(newRoot)
			app.clearPath(archive, source.fullPath())
			clone := source.clone()
			folder := archive.findFile(source.path())
			folder.children = append(folder.children, clone)
			folder.sorted = false
			clone.parent = folder
			clone.state = hashed
			source.state = pending
		}
		app.fs.Copy(filepath.Join(source.fullPath()...), app.curArchive.rootPath, roots...)
	}
}

func (app *appState) delete(sourceArcIdx int, source *file) {
	if source.state != divergent && source.folder != nil {
		return
	}

	sameHash := make([][]*file, len(app.archives))
	for arcIdx, archive := range app.archives {
		archive.rootFolder.walk(func(_ int, child *file) handleResult {
			if child.hash == source.hash {
				sameHash[arcIdx] = append(sameHash[arcIdx], child)
			}
			return advance
		})
	}

	if len(sameHash[0]) > 0 {
		return
	}

	for arcIdx, files := range sameHash {
		archive := app.archives[arcIdx]
		for _, file := range files {
			archive.deleteFile(file)
			app.fs.Delete(filepath.Join(archive.rootPath, filepath.Join(file.fullPath()...)))
		}
	}
}

func (app *appState) clearPath(archive *archive, path []string) {
	folder := archive.rootFolder
	var child *file
	for _, name := range path {
		child = folder.findChild(name)
		if child == nil {
			return
		}
		if child.folder == nil {
			newName := folder.uniqueName(child.name)
			newPath := slices.Clone(child.fullPath())
			newPath[len(newPath)-1] = newName
			app.fs.Rename(archive.rootPath, filepath.Join(child.fullPath()...), filepath.Join(newPath...))
			child.name = newName
			folder.sorted = false
			return
		}
		folder = child
	}
	if child != nil && child.folder != nil {
		folder = child.parent
		newName := folder.uniqueName(child.name)
		newPath := slices.Clone(child.fullPath())
		newPath[len(newPath)-1] = newName
		app.fs.Rename(archive.rootPath, filepath.Join(child.fullPath()...), filepath.Join(newPath...))
		child.name = newName
		folder.sorted = false
		return
	}
}

func (folder *folder) uniqueName(name string) string {
loop:
	for i := 1; ; i++ {
		name = newSuffix(name, i)
		for _, child := range folder.children {
			if child.name == name {
				continue loop
			}
		}
		break
	}
	return name
}

func newSuffix(name string, idx int) string {
	parts := strings.Split(name, ".")

	var part string
	if len(parts) == 1 {
		part = stripIdx(parts[0])
	} else {
		part = stripIdx(parts[len(parts)-2])
	}
	var newName string
	if len(parts) == 1 {
		newName = fmt.Sprintf("%s%c%d", part, '`', idx)
	} else {
		parts[len(parts)-2] = fmt.Sprintf("%s%c%d", part, '`', idx)
		newName = strings.Join(parts, ".")
	}
	return newName
}

type stripIdxState int

const (
	expectDigit stripIdxState = iota
	expectDigitOrBacktick
)

func stripIdx(name string) string {
	state := expectDigit
	i := len(name) - 1
	for ; i >= 0; i-- {
		ch := name[i]
		if ch >= '0' && ch <= '9' && (state == expectDigit || state == expectDigitOrBacktick) {
			state = expectDigitOrBacktick
		} else if ch == '`' && state == expectDigitOrBacktick {
			return name[:i]
		} else {
			return name
		}
	}
	return name
}
