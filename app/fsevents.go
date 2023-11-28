package app

import (
	"arc/fs"
	"arc/log"
	"fmt"
	"strings"
)

func (app *appState) handleFsEvent(event fs.Event) {
	switch event := event.(type) {
	case fs.FileMeta:
		archive := app.archive(event.Root)
		path, name := parseName(event.Path)
		incoming := &file{
			archive: archive,
			name:    name,
			size:    event.Size,
			modTime: event.ModTime,
			hash:    event.Hash,
			state:   scanned,
		}
		if event.Hash != "" {
			incoming.state = hashed
		}
		folder := archive.getFile(path)
		folder.children = append(folder.children, incoming)
		incoming.parent = folder
		folder.sorted = false

	case fs.FileHashed:
		file := app.archive(event.Root).findFile(parsePath(event.Path))
		file.hash = event.Hash
		file.state = hashed
		app.archive(event.Root).archiveState = archiveScanned

	case fs.ArchiveHashed:
		log.Debug("hashed", "archive", event.Root)
		app.archive(event.Root).archiveState = archiveHashed
		app.analyze()

	case fs.CopyProgress:
		file := app.archive(event.Root).findFile(parsePath(event.Path))
		file.state = copying
		file.copied = event.Copyed

	case fs.Copied:
		file := app.archive(event.FromRoot).findFile(parsePath(event.Path))
		file.state = copied
		file.copied = file.size
		app.analyze()

	case fs.Renamed, fs.Deleted:
		app.analyze()

	default:
		log.Debug("handleFsEvent", "unhandled", fmt.Sprintf("%T", event))
		panic(event)
	}
}

func (app *appState) analyze() {
	for _, arc := range app.archives {
		if arc.archiveState != archiveHashed {
			return
		}
	}

	countsByHash := map[string][]int{}
	copyingInProgress := false
	for i, arc := range app.archives {
		arc.rootFolder.walk(func(_ int, file *file) handleResult {
			if file.state == pending || file.state == copying {
				copyingInProgress = true
			}
			if file.state != pending {
				if file.hash == "" {
					file.state = scanned
				} else {
					file.state = hashed
				}
			}
			path := file.fullPath()
			for j, otherArc := range app.archives {
				if i == j {
					continue
				}
				otherFile := otherArc.findFile(path)
				if otherFile == nil || otherFile.hash != file.hash {
					file.state = divergent
					break
				}
			}
			file.counts = countsByHash[file.hash]
			if file.counts == nil {
				file.counts = make([]int, len(app.archives))
				countsByHash[file.hash] = file.counts
			}
			file.counts[i]++
			return advance
		})
	}

	for i, arc := range app.archives {
		arc.rootFolder.walk(func(_ int, file *file) handleResult {
			if !copyingInProgress && file.state == copied {
				file.state = hashed
				file.copying = 0
				file.copied = 0
			}
			if file.state == divergent {
				return advance
			}
			counts := countsByHash[file.hash][i]
			if counts > 1 {
				file.state = duplicate
			}
			return advance
		})
	}
}

func parsePath(strPath string) []string {
	if strPath == "" {
		return nil
	}
	return strings.Split(strPath, "/")
}

func parseName(strPath string) ([]string, string) {
	path := parsePath(strPath)
	base := path[len(path)-1]
	return path[:len(path)-1], base
}
