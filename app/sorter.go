package app

import (
	"cmp"
	"slices"
	"strings"
)

func (app *appState) sort() {
	folder := app.curArchive.curFolder
	if folder.sorted || len(folder.children) == 0 {
		return
	}
	folder.sorted = true

	entries := folder.children
	switch folder.sortColumn {
	case sortByName:
		entries.sortByName()
	case sortByTime:
		entries.sortByTime()
	case sortBySize:
		entries.sortBySize()
	}
	if !folder.sortAscending[folder.sortColumn] {
		entries.reverse()
	}
	app.makeSelectedVisible = true
}

func (e files) sortByName() {
	slices.SortFunc(e, func(i, j *file) int {
		byName := cmp.Compare(strings.ToLower(i.name), strings.ToLower(j.name))
		if byName != 0 {
			return byName
		}
		byTime := cmp.Compare(i.size, j.size)
		if byTime != 0 {
			return byTime
		}
		return i.modTime.Compare(j.modTime)
	})
}

func (e files) sortBySize() {
	slices.SortFunc(e, func(i, j *file) int {
		bySize := cmp.Compare(i.size, j.size)
		if bySize != 0 {
			return bySize
		}
		byName := cmp.Compare(strings.ToLower(i.name), strings.ToLower(j.name))
		if byName != 0 {
			return byName
		}
		return i.modTime.Compare(j.modTime)
	})
}

func (e files) sortByTime() {
	slices.SortFunc(e, func(i, j *file) int {
		byTime := i.modTime.Compare(j.modTime)
		if byTime != 0 {
			return byTime
		}
		byName := cmp.Compare(strings.ToLower(i.name), strings.ToLower(j.name))
		if byName != 0 {
			return byName
		}
		return cmp.Compare(i.size, j.size)
	})
}

func (e files) reverse() {
	slices.Reverse(e)
}
