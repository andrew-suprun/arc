package app

import (
	"arc/fs"
	"arc/log"
	"fmt"
	"strings"
)

func (app *appState) handleFsEvent(event fs.Event) {
	switch event := event.(type) {
	case fs.FileMetas:
		for _, meta := range event {
			app.addFileMeta(meta)
		}

	case fs.FileHashed:
		path, name := parseName(event.Path)
		file := app.archive(event.Root).getFolder(path).children.getFile(name)
		file.hash = event.Hash
		file.progress = file.size
		file.state = hashed

	case fs.Progress:
		path, name := parseName(event.Path)
		folder := app.archive(event.Root).getFolder(path)
		file := folder.children.getFile(name)
		file.progress = event.Progress
		file.state = inProgress

	case fs.Quit:
		app.quit = true
	default:
		log.Debug("handleFsEvent", "unhandled", fmt.Sprintf("%T", event))
	}
}

func (app *appState) addFileMeta(meta fs.FileMeta) {
	archive := app.archive(meta.Root)
	path, name := parseName(meta.Path)
	incoming := &file{
		name:    name,
		size:    meta.Size,
		modTime: meta.ModTime,
		hash:    meta.Hash,
		state:   scanned,
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
	if strPath == "" {
		return nil
	}
	return strings.Split(string(strPath), "/")
}

func parseName(strPath string) ([]string, string) {
	path := parsePath(strPath)
	base := path[len(path)-1]
	return path[:len(path)-1], base
}
