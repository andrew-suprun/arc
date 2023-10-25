package mockfs

import (
	"arc/fs"
	"arc/stream"
	"cmp"
	"encoding/csv"
	"os"
	"slices"
	"strconv"
	"sync/atomic"
	"time"
)

type fsys struct {
	scan     bool
	commands *stream.Stream[command]
	events   chan fs.Event
	quit     atomic.Bool
}

type command interface {
	command()
}

type (
	scan struct{ root string }
	copy struct {
		path     string
		fromRoot string
		toRoots  []string
	}
	rename struct {
		root       string
		sourcePath string
		targetPath string
	}
	delete struct {
		path string
	}
)

func (scan) command()   {}
func (copy) command()   {}
func (rename) command() {}
func (delete) command() {}

func NewFS(scan bool) fs.FS {
	fs := &fsys{
		scan:     scan,
		commands: stream.NewStream[command]("commands"),
		events:   make(chan fs.Event, 256),
	}
	go fs.run()
	return fs
}

func (fs *fsys) Events() <-chan fs.Event {
	return fs.events
}

func (fs *fsys) Scan(root string) {
	fs.commands.Push(scan{root: root})
}

func (fs *fsys) Copy(path, fromRoot string, toRoots ...string) {
	fs.commands.Push(copy{path: path, fromRoot: fromRoot, toRoots: toRoots})
}

func (fs *fsys) Rename(root, sourcePath, targetPath string) {
	fs.commands.Push(rename{root: root, sourcePath: sourcePath, targetPath: targetPath})
}

func (fs *fsys) Delete(path string) {
	fs.commands.Push(delete{path: path})
}

func (f *fsys) Quit() {
	f.quit.Store(true)
	f.events <- fs.Quit{}
}

func (f *fsys) run() {
	for !f.quit.Load() {
		commands, _ := f.commands.Pull()
		for _, command := range commands {
			switch cmd := command.(type) {
			case scan:
				f.scanArchive(cmd)
			case copy:
				f.copyFile(cmd)
			case rename:
				f.renameFile(cmd)
			case delete:
				f.deleteFile(cmd)
			}
		}
	}
}

func (f *fsys) scanArchive(scan scan) {
	f.events <- archives[scan.root]
	if f.scan {
		for _, file := range archives[scan.root] {
			if f.quit.Load() {
				return
			}

			for progress := 0; progress < file.Size; progress += 10000000 {
				if f.quit.Load() {
					return
				}
				f.events <- fs.Progress{
					Root:     scan.root,
					Path:     file.Path,
					Progress: progress,
				}
				time.Sleep(100 * time.Microsecond)
			}
			f.events <- fs.FileHashed{
				Root: scan.root,
				Path: file.Path,
				Hash: file.Hash,
			}
		}
	}

	f.events <- fs.ArchiveHashed{Root: scan.root}
}

func (fs *fsys) copyFile(copy copy) {
}

func (fs *fsys) renameFile(rename rename) {
}

func (fs *fsys) deleteFile(delete delete) {
}

var archives = map[string]fs.FileMetas{}

func init() {
	or := readMeta()
	for i := range or {
		or[i].Root = "origin"
	}
	c1 := slices.Clone(or)
	for i := range c1 {
		c1[i].Root = "copy 1"
	}
	c2 := slices.Clone(or)
	for i := range c1 {
		c2[i].Root = "copy 2"
	}
	archives = map[string]fs.FileMetas{
		"origin": or,
		"copy 1": c1,
		"copy 2": c2,
	}
}

func readMeta() fs.FileMetas {
	result := fs.FileMetas{}
	hashInfoFile, err := os.Open("data/.meta.csv")
	if err != nil {
		return nil
	}
	defer hashInfoFile.Close()

	records, err := csv.NewReader(hashInfoFile).ReadAll()
	if err != nil || len(records) == 0 {
		return nil
	}

	for _, record := range records[1:] {
		if len(record) == 5 {
			name := record[1]
			size, er2 := strconv.ParseUint(record[2], 10, 64)
			modTime, er3 := time.Parse(time.RFC3339, record[3])
			modTime = modTime.UTC().Round(time.Second)
			hash := record[4]
			if hash == "" || er2 != nil || er3 != nil {
				continue
			}

			result = append(result, fs.FileMeta{
				Path:    name,
				Hash:    hash,
				Size:    int(size),
				ModTime: modTime,
			})
		}
	}
	slices.SortFunc(result, func(a, b fs.FileMeta) int {
		return cmp.Compare(a.Path, b.Path)
	})
	return result
}
