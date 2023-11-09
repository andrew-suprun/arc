package app

import (
	"arc/fs"
	"fmt"
	"slices"
	"strings"
	"time"
)

type (
	appState struct {
		fs         fs.FS
		archives   []*archive
		curArchive *archive

		screenWidth   int
		screenHeight  int
		folderTargets []folderTarget
		sortTargets   []sortTarget
		lastClickTime time.Time

		makeSelectedVisible bool
		sync                bool
		quit                bool
	}

	archive struct {
		rootPath     string
		rootFolder   *file
		curFolder    *file
		archiveState archiveState
	}

	file struct {
		name     string
		size     int
		modTime  time.Time
		hash     string
		progress int
		state    fileState
		parent   *file
		counts   []int
		*folder
	}

	files []*file

	folder struct {
		children      files
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
	archiveScanned archiveState = iota
	archiveHashed
)

const (
	scanned fileState = iota
	inProgress
	hashed
	pending
	copied
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

func (app *appState) curArciveIdx() int {
	for i := range app.archives {
		if app.curArchive == app.archives[i] {
			return i
		}
	}
	panic("Invalid current archive")
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
	if f.progress > 0 && f.progress < f.size {
		fmt.Fprintf(buf, ", progress: %d", f.progress)
	}
	buf.WriteRune('}')
	return buf.String()
}

func (s fileState) String() string {
	switch s {
	case scanned:
		return "scanned"
	case inProgress:
		return "inProgress"
	case hashed:
		return "hashed"
	case pending:
		return "pending"
	case copied:
		return "copied"
	case divergent:
		return "divergent"
	}
	panic("Invalid state")
}

func (parent *file) getSub(sub string) *file {
	for _, child := range parent.children {
		if child.name == sub {
			return child
		}
	}
	child := &file{
		name:   sub,
		state:  scanned,
		parent: parent,
		folder: &folder{
			sortAscending: []bool{true, true, true},
		},
	}
	parent.children = append(parent.children, child)
	parent.sorted = false
	return child
}

func (arc *archive) getFolder(path []string) *file {
	folder := arc.rootFolder
	for _, sub := range path {
		folder = folder.getSub(sub)
	}
	return folder
}

func (f *file) clone() *file {
	return &file{
		name:     f.name,
		size:     f.size,
		modTime:  f.modTime,
		hash:     f.hash,
		progress: f.progress,
		state:    f.state,
		counts:   f.counts,
	}
}

func (f *file) getChild(name string) *file {
	for _, file := range f.children {
		if file.name == name {
			return file
		}
	}
	return nil
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
	folder.progress = 0

	for _, child := range folder.children {
		if child.folder != nil {
			child.updateMetas()
		}
		folder.updateMeta(child)
	}
	if folder.size == 0 {
		folder.parent.deleteFile(folder)
	}
}

func (folder *file) updateMeta(meta *file) {
	folder.progress += meta.progress
	folder.size += meta.size
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
	folder := arc.getFolder(file.path())
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
