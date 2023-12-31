package app

import (
	"arc/fs"
	"arc/lifecycle"
	"fmt"
	"slices"
	"strings"
	"time"
)

type (
	appState struct {
		lc *lifecycle.Lifecycle

		fs         fs.FS
		archives   []*archive
		curArchive *archive

		screenWidth   int
		screenHeight  int
		folderTargets []folderTarget
		sortTargets   []sortTarget
		lastClickTime time.Time
		lastX         width
		lastY         int

		makeSelectedVisible bool
		sync                bool
	}

	archive struct {
		idx          int
		rootPath     string
		rootFolder   *file
		curFolder    *file
		archiveState archiveState
		nDuplicates  int
		nDivergents  int
	}

	file struct {
		archive *archive
		name    string
		size    int
		modTime time.Time
		hash    string
		copying int
		copied  int
		state   fileState
		parent  *file
		counts  []int
		*folder
	}

	files []*file

	folder struct {
		children      files
		selected      *file
		nFiles        int
		nHashed       int
		selectedIdx   int
		offsetIdx     int
		sortColumn    sortColumn
		sortAscending []bool
		sorted        bool
	}

	archiveState int
	fileState    int
	sortColumn   int

	folderTarget struct {
		path   []string
		offset width
		width  width
	}

	sortTarget struct {
		sortColumn
		offset width
		width  width
	}
)

const (
	archiveStarted archiveState = iota
	archiveScanned
	archiveHashed
)

const (
	scanned fileState = iota
	hashed
	pending
	copying
	copied
	duplicate
	divergent
)

const (
	sortByName sortColumn = iota
	sortByTime
	sortBySize
)

func (app *appState) archive(root string) *archive {
	for _, archive := range app.archives {
		if archive.rootPath == root {
			return archive
		}
	}
	return nil
}

func (app *appState) getSelected() *file {
	return app.curArchive.curFolder.getSelected()
}

func (app *appState) setSelected(file *file) {
	app.curArchive = file.archive
	app.curArchive.curFolder = file.archive.findFile(file.path())
	app.curArchive.curFolder.selected = file
}

func (f *file) String() string {
	if f == nil {
		return "<nil>"
	}
	buf := &strings.Builder{}
	if f.folder == nil {
		buf.WriteString("file")
	} else {
		buf.WriteString("folder")
	}
	fmt.Fprintf(buf, "{name: %q, path: %v, state: %s, size: %d, modTime: %s", f.name, f.path(), f.state, f.size, f.modTime.Format(time.DateTime))
	if f.hash != "" {
		fmt.Fprintf(buf, ", hash: %q", f.hash)
	}
	if f.folder != nil {
		fmt.Fprintf(buf, ", nFiles: %d, nHashed: %d", f.nFiles, f.nHashed)
	}
	if f.copying > 0 {
		fmt.Fprintf(buf, ", copying: %d, copied: %d", f.copying, f.copied)
	}
	buf.WriteRune('}')
	return buf.String()
}

func (s fileState) String() string {
	switch s {
	case scanned:
		return "scanned"
	case hashed:
		return "hashed"
	case pending:
		return "pending"
	case copying:
		return "copying"
	case copied:
		return "copied"
	case duplicate:
		return "duplicate"
	case divergent:
		return "divergent"
	}
	panic("Invalid state")
}

func (f *file) findChild(name string) *file {
	if f.folder == nil {
		return nil
	}
	for _, file := range f.children {
		if file.name == name {
			return file
		}
	}
	return nil
}

func (folder *file) getSelected() *file {
	if folder.selectedIdx >= len(folder.children) {
		folder.selectedIdx = len(folder.children) - 1
	}
	if folder.selectedIdx < 0 {
		folder.selectedIdx = 0
	}
	if folder.selected != nil {
		for i, child := range folder.children {
			if child == folder.selected {
				folder.selectedIdx = i
			}
		}
	}
	if len(folder.children) == 0 {
		return nil
	}
	folder.selected = folder.children[folder.selectedIdx]
	return folder.selected
}

func (parent *file) getChild(sub string) *file {
	child := parent.findChild(sub)
	if child == nil {
		child = &file{
			archive: parent.archive,
			name:    sub,
			state:   scanned,
			parent:  parent,
			folder: &folder{
				sortAscending: []bool{true, true, true},
			},
		}
		parent.children = append(parent.children, child)
		parent.sorted = false
	}
	return child
}

func (arc *archive) findFile(path []string) *file {
	file := arc.rootFolder
	for _, sub := range path {
		file = file.findChild(sub)
		if file == nil {
			return nil
		}
	}
	return file
}

func (arc *archive) getFile(path []string) *file {
	folder := arc.rootFolder
	for _, sub := range path {
		folder = folder.getChild(sub)
	}
	return folder
}

func (f *file) clone(archive *archive) *file {
	return &file{
		archive: archive,
		name:    f.name,
		size:    f.size,
		modTime: f.modTime,
		hash:    f.hash,
		copied:  f.copied,
		state:   f.state,
		counts:  f.counts,
	}
}

func (f *file) path() (result []string) {
	for f.parent != nil {
		f = f.parent
		if f.name == "" {
			break
		}
		result = append(result, f.name)
	}
	slices.Reverse(result)
	return result
}

func (f *file) fullPath() (result []string) {
	result = append(result, f.name)
	for f.parent != nil {
		f = f.parent
		if f.name == "" {
			break
		}
		result = append(result, f.name)
	}
	slices.Reverse(result)
	return result
}

func (folder *file) updateMetas() {
	folder.size = 0
	folder.modTime = time.Time{}
	folder.state = scanned
	folder.copying = 0
	folder.copied = 0
	folder.nFiles = 0
	folder.nHashed = 0

	for _, child := range folder.children {
		if child.folder != nil {
			child.updateMetas()
			folder.nFiles += child.nFiles - 1
			folder.nHashed += child.nHashed
		}
		folder.updateMeta(child)
	}
	if folder.size == 0 && folder.parent != nil {
		folder.parent.deleteFile(folder)
	}
}

func (folder *file) updateMeta(meta *file) {
	folder.size += meta.size
	folder.copying += meta.copying
	folder.copied += meta.copied
	folder.nFiles++
	if meta.hash != "" {
		folder.nHashed++
	}
	if folder.modTime.Before(meta.modTime) {
		folder.modTime = meta.modTime
	}
	folder.state = max(folder.state, meta.state)
}

type handleResult int

const (
	advance handleResult = iota
	stop
)

func (folder *file) walk(handle func(int, *file) handleResult) (result handleResult) {
	for idx, child := range folder.children {
		if child.folder != nil {
			result = child.walk(handle)
		} else {
			result = handle(idx, child)
		}
		if result == stop {
			break
		}
	}
	return result
}

func (arc *archive) deleteFile(file *file) {
	folder := arc.findFile(file.path())
	folder.deleteFile(file)
}

func (folder *folder) deleteFile(file *file) {
	for childIdx, child := range folder.children {
		if child == file {
			folder.children = slices.Delete(folder.children, childIdx, childIdx+1)
			break
		}
	}
}

func (app *appState) state() archiveState {
	state := archiveHashed
	for _, archive := range app.archives {
		if state > archive.archiveState {
			state = archive.archiveState
		}
	}
	return state
}
