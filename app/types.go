package app

import (
	"arc/fs"
	"time"
)

type (
	appState struct {
		fs         fs.FS
		archives   []*archive
		curArchive *archive

		screenWidth  int
		screenHeight int

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
