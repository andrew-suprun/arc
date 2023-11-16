package app

import (
	"arc/fs"
	"arc/log"
	"fmt"
	"slices"
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
		file := app.archive(event.Root).getFolder(path).getChild(name)
		file.hash = event.Hash
		file.progress = file.size
		file.state = hashed

	case fs.Progress:
		path, name := parseName(event.Path)
		folder := app.archive(event.Root).getFolder(path)
		file := folder.getChild(name)
		file.progress = event.Progress
		file.state = inProgress

	case fs.ArchiveHashed:
		app.archive(event.Root).archiveState = archiveHashed
		app.analyze()

	case fs.Copied:
		path, name := parseName(event.Path)
		app.archive(event.FromRoot).getFolder(path).getChild(name).state = hashed

	case fs.Renamed, fs.Deleted:
		// Do nothing

	case fs.Quit:
		app.quit = true

	default:
		log.Debug("handleFsEvent", "unhandled", fmt.Sprintf("%T", event))
		panic(event)
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

func (app *appState) analyze() {
	for _, arc := range app.archives {
		if arc.archiveState != archiveHashed {
			return
		}
	}
	byHash := map[string][][]*file{}
	for i, archive := range app.archives {
		app.collectFilesByHash(i, archive.rootFolder, byHash)
	}

	for hash, files := range byHash {
		analyzeDiscrepancy(hash, files)
	}
}

func (app *appState) collectFilesByHash(arcIdx int, folder *file, byHash map[string][][]*file) {
	for _, child := range folder.children {
		if child.folder != nil {
			app.collectFilesByHash(arcIdx, child, byHash)
		} else {
			sameHash := byHash[child.hash]
			if sameHash == nil {
				sameHash = make([][]*file, len(app.archives))
				byHash[child.hash] = sameHash
			}
			sameHash[arcIdx] = append(sameHash[arcIdx], child)
		}
	}
}

func analyzeDiscrepancy(hash string, files [][]*file) {
	discrepancy := false
	for _, arc := range files {
		if len(arc) != 1 {
			discrepancy = true
			break
		}
	}

	if !discrepancy {
		origin := files[0]
		for _, copy := range files[1:] {
			if origin[0].name != copy[0].name ||
				!slices.Equal(origin[0].path(), copy[0].path()) {

				discrepancy = true
				break
			}
		}
	}

	if discrepancy {
		counts := make([]int, len(files))
		for i := range files {
			counts[i] = len(files[i])
		}
		for _, arc := range files {
			for _, file := range arc {
				file.state = divergent
				file.counts = counts
			}
		}
	}
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
