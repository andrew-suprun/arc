package app

import (
	"arc/fs"
	"arc/log"
	"strings"
)

func (app *appState) handleFsEvent(event fs.Event) {
	switch event := event.(type) {
	case fs.FileMetas:
		for _, meta := range event {
			app.addFileMeta(meta)
		}
	case fs.FileMeta:
		app.addFileMeta(event)
	}
}

func (app *appState) addFileMeta(meta fs.FileMeta) {
	log.Debug("app", "addFile", meta)
	archive := app.archive(meta.Root)
	path, name := parseName(meta.Path)
	incoming := &file{
		name:    name,
		size:    meta.Size,
		modTime: meta.ModTime,
		hash:    meta.Hash,
		state:   resolved,
	}
	folder := archive.rootFolder
	for _, sub := range path {
		folder = folder.getSub(sub)
	}
	folder.children = append(folder.children, incoming)
	incoming.parent = folder
	folder.sorted = false
}

func parsePath(strPath string) []string {
	return strings.Split(string(strPath), "/")
}

func parseName(strPath string) ([]string, string) {
	path := parsePath(strPath)
	base := path[len(path)-1]
	return path[:len(path)-1], base
}
