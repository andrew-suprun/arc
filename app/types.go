package app

import (
	"arc/fs"
	"arc/log"
	"path/filepath"
	"slices"
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
		rootPath   string
		rootFolder *file
		curFolder  *file
	}

	file struct {
		name     string
		size     int
		modTime  time.Time
		hash     string
		progress int
		state    fileState
		parent   *file
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

	fileState  int
	sortColumn int

	folderTarget struct {
		path   []string
		offset int
		width  int
	}

	sortTarget struct {
		sortColumn
		offset int
		width  int
	}
)

const (
	resolved fileState = iota
	hashing
	hashed
	pending
	copying
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

func (parent *file) getSub(sub string) *file {
	for _, child := range parent.children {
		if child.name == sub {
			return child
		}
	}
	child := &file{
		name:   sub,
		state:  resolved,
		parent: parent,
	}
	parent.children = append(parent.children, child)
	parent.sorted = false
	return child
}

func (app *appState) getFolder(path []string) *file {
	folder := app.curArchive.rootFolder
	for _, sub := range path {
		folder.children.getFile(sub)
	}
	return folder
}

func (f files) getFile(name string) *file {
	for _, file := range f {
		if file.name == name {
			return file
		}
	}
	log.Panic("File does not exists", "name", name)
	return nil
}

func (f *file) path() (result []string) {
	for f.parent != nil {
		result = append(result, f.parent.name)
	}
	slices.Reverse(result)
	return result
}

func (m *file) fullPath() string {
	if m.parent == nil {
		return ""
	}
	segments := []string{}
	m = m.parent
	for m.parent != nil {
		segments = append(segments, m.name)
		m = m.parent
	}

	slices.Reverse(segments)
	return filepath.Join(segments...)
}
