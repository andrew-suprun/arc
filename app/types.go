package app

import (
	"arc/fs"
	"arc/log"
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
		folder = folder.children.getFile(sub)
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
		f = f.parent
	}
	result = result[:len(result)-1]
	slices.Reverse(result)
	return result
}

func (f *file) fullPath() (result []string) {
	result = append(result, f.name)
	for f.parent != nil {
		result = append(result, f.parent.name)
		f = f.parent
	}
	result = result[:len(result)-1]
	slices.Reverse(result)
	return result
}
