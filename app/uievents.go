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
		folder := app.curArchive.curFolder
		folder.selectedIdx--
		folder.selected = nil
		app.makeSelectedVisible = true

	case "Down":
		folder := app.curArchive.curFolder
		folder.selectedIdx++
		folder.selected = nil
		app.makeSelectedVisible = true

	case "PgUp":
		folder := app.curArchive.curFolder
		folder.selectedIdx -= app.screenHeight - 4
		folder.selected = nil
		folder.offsetIdx -= app.screenHeight - 4

	case "PgDn":
		folder := app.curArchive.curFolder
		folder.selectedIdx += app.screenHeight - 4
		folder.selected = nil
		folder.offsetIdx += app.screenHeight - 4

	case "Home":
		folder := app.curArchive.curFolder
		folder.selectedIdx = 0
		folder.selected = nil
		app.makeSelectedVisible = true

	case "End":
		folder := app.curArchive.curFolder
		folder.selectedIdx = len(folder.children) - 1
		folder.selected = nil
		app.makeSelectedVisible = true

	case "Right":
		folder := app.curArchive.curFolder
		child := folder.getSelected()
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
		name := folder.getSelected().name
		path := filepath.Join(archive.rootPath, filepath.Join(folder.fullPath()...), name)
		exec.Command("open", "-R", path).Start()

	case "Enter":
		archive := app.curArchive
		folder := archive.curFolder
		name := folder.getSelected().name
		path := filepath.Join(archive.rootPath, filepath.Join(folder.fullPath()...), name)
		exec.Command("open", path).Start()

	case "Ctrl+C":
		app.fs.Quit()

	case "Ctrl+R":
		app.resolve(app.curArchive.curFolder.getSelected())

	case "Ctrl+A":
		app.resolve(app.curArchive.curFolder)

	case "Tab":
		_, next := app.findNeighbours()
		if next != nil {
			app.setSelected(next)
			app.makeSelectedVisible = true
		}

	case "Backtab":
		prev, _ := app.findNeighbours()
		if prev != nil {
			app.setSelected(prev)
			app.makeSelectedVisible = true
		}

	case "Backspace2": // Ctrl+Delete
		folder := app.curArchive.curFolder
		app.delete(folder.getSelected())

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
			folder.selected = nil
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

func (app *appState) findNeighbours() (prev, next *file) {
	var foundSameFile bool
	cur := app.getSelected()
	for _, archive := range app.archives {
		if archive.rootFolder.walk(func(_ int, f *file) handleResult {
			if f.hash != cur.hash {
				return advance
			}
			if f == cur {
				foundSameFile = true
			} else if foundSameFile {
				next = f
				return stop
			} else {
				prev = f
			}
			return advance
		}) == stop {
			break
		}
	}
	return prev, next
}

func (app *appState) resolve(source *file) {
	if source.state != divergent {
		return
	}
	if source.folder != nil {
		for _, child := range source.children {
			app.resolve(child)
		}
		return
	}

	path := source.fullPath()
	archives := []*archive{}
	for _, archive := range app.archives {
		if archive == source.archive {
			continue
		}
		otherFile := archive.findFile(path)
		if otherFile != nil && otherFile.hash == source.hash {
			continue
		}
		app.clearPath(archive, source.fullPath())

		renamed := false
		archive.rootFolder.walk(func(_ int, child *file) handleResult {
			if child.hash == source.hash && child.state == divergent {
				archive.deleteFile(child)
				clone := source.clone(archive)
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
			archives = append(archives, archive)
		}
	}

	if len(archives) > 0 {
		for _, archive := range archives {
			app.clearPath(archive, source.fullPath())
			clone := source.clone(archive)
			folder := archive.getFile(source.path())
			folder.children = append(folder.children, clone)
			folder.sorted = false
			clone.parent = folder
			clone.state = hashed
			source.state = pending
			source.counts[archive.idx]++
		}

		roots := make([]string, len(archives))
		for i := range archives {
			roots[i] = archives[i].rootPath
		}

		app.fs.Copy(filepath.Join(source.fullPath()...), app.curArchive.rootPath, roots...)
	}
}

func (app *appState) delete(source *file) {
	if source.folder != nil {
		return
	}
	path := source.fullPath()
	for _, archive := range app.archives {
		file := archive.getFile(path)
		if file != nil && file.hash == source.hash {
			archive.deleteFile(file)
			app.fs.Delete(filepath.Join(archive.rootPath, filepath.Join(file.fullPath()...)))
			file.counts[archive.idx]--
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
