package mockfs

import (
	"arc/fs"
	"arc/lifecycle"
	"arc/log"
	"arc/stream"
	"cmp"
	"encoding/csv"
	"os"
	"slices"
	"strconv"
	"time"
)

type fsys struct {
	lc       *lifecycle.Lifecycle
	scan     bool
	commands *stream.Stream[command]
	events   chan fs.Event
}

type command interface {
	command()
}

type (
	scan struct{ root string }
	copy struct {
		path     string
		hash     string
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

func NewFS(lc *lifecycle.Lifecycle, scan bool) fs.FS {
	fs := &fsys{
		lc:       lc,
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

func (fs *fsys) Copy(path, hash, fromRoot string, toRoots ...string) {
	fs.commands.Push(copy{path: path, hash: hash, fromRoot: fromRoot, toRoots: toRoots})
}

func (fs *fsys) Rename(root, sourcePath, targetPath string) {
	fs.commands.Push(rename{root: root, sourcePath: sourcePath, targetPath: targetPath})
}

func (fs *fsys) Delete(path string) {
	fs.commands.Push(delete{path: path})
}

func (f *fsys) Quit() {
	f.commands.Close()
	f.lc.Stop()
}

func (f *fsys) run() {
	for {
		for _, command := range f.commands.Pull() {
			if f.lc.ShoudStop() {
				return
			}
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
	metas := archives[scan.root]
	for i := range metas {
		meta := metas[i]
		meta.Hash = ""
		f.events <- meta
	}
	for _, file := range metas {
		f.events <- fs.FileHashed{
			Root: scan.root,
			Path: file.Path,
			Hash: file.Hash,
		}
		if f.scan {
			time.Sleep(time.Millisecond)
		}
	}

	f.events <- fs.ArchiveHashed{Root: scan.root}
}

func (f *fsys) copyFile(copy copy) {
	log.Debug("copy", "path", copy.path, "from", copy.fromRoot, "to", copy.toRoots)
	archive := archives[copy.fromRoot]
	var file fs.FileMeta
	for _, file = range archive {
		if file.Path == copy.path {
			break
		}
	}
	for progress := 0; progress < file.Size; progress += 100000 {
		if f.lc.ShoudStop() {
			return
		}
		f.events <- fs.CopyProgress{
			Root:   copy.fromRoot,
			Path:   copy.path,
			Copyed: progress,
		}
		time.Sleep(time.Millisecond)
	}
	f.events <- fs.Copied{Path: copy.path, FromRoot: copy.fromRoot, ToRoots: copy.toRoots}
}

func (f *fsys) renameFile(rename rename) {
	log.Debug("rename", "root", rename.root, "source", rename.sourcePath, "target", rename.targetPath)
	f.events <- fs.Renamed{Root: rename.root, SourcePath: rename.sourcePath, TargetPath: rename.targetPath}
}

func (f *fsys) deleteFile(delete delete) {
	log.Debug("delete", "path", delete.path)
	f.events <- fs.Deleted{Path: delete.path}
}

var archives = map[string][]fs.FileMeta{}

func init() {
	or := readMeta()
	// or := []fs.FileMeta{}

	c1 := slices.Clone(or)
	c2 := slices.Clone(or)

	or = append(or, fs.FileMeta{
		Path:    "aaa/bbb/ccc",
		Size:    11111111,
		ModTime: time.Now(),
		Hash:    "ccc",
	})

	or = append(or, fs.FileMeta{
		Path:    "bbb",
		Size:    12300000,
		ModTime: time.Now(),
		Hash:    "bbb",
	})

	or = append(or, fs.FileMeta{
		Path:    "xxx",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	or = append(or, fs.FileMeta{
		Path:    "yyy",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	or = append(or, fs.FileMeta{
		Path:    "zzz",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm1/aaa",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm1/aaa",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm1/bbb",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm1/bbb",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm1/ccc",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm1/ccc",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm2/aaa",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm2/aaa",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm2/bbb",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm2/bbb",
	})

	or = append(or, fs.FileMeta{
		Path:    "nnn/mmm2/ccc",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "nnn/mmm2/ccc",
	})

	c1 = append(c1, fs.FileMeta{
		Path:    "bbb",
		Size:    11111111,
		ModTime: time.Now(),
		Hash:    "ccc",
	})

	c1 = append(c1, fs.FileMeta{
		Path:    "aaa/bbb/ccc",
		Size:    12300000,
		ModTime: time.Now(),
		Hash:    "bbb",
	})

	c1 = append(c1, fs.FileMeta{
		Path:    "xxx",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	c1 = append(c1, fs.FileMeta{
		Path:    "yyy",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	c2 = append(c2, fs.FileMeta{
		Path:    "aaa/bbb",
		Size:    23400000,
		ModTime: time.Now(),
		Hash:    "222",
	})

	c2 = append(c2, fs.FileMeta{
		Path:    "ddd/eee",
		Size:    12300000,
		ModTime: time.Now(),
		Hash:    "111",
	})

	c2 = append(c2, fs.FileMeta{
		Path:    "ddd/fff",
		Size:    33300000,
		ModTime: time.Now(),
		Hash:    "333",
	})

	c2 = append(c2, fs.FileMeta{
		Path:    "xxx",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	c2 = append(c2, fs.FileMeta{
		Path:    "yyy",
		Size:    99900000,
		ModTime: time.Now(),
		Hash:    "xxx",
	})

	for i := range or {
		or[i].Root = "origin"
	}
	for i := range c1 {
		c1[i].Root = "copy 1"
	}
	for i := range c2 {
		c2[i].Root = "copy 2"
	}

	archives = map[string][]fs.FileMeta{
		"origin": or,
		"copy 1": c1,
		"copy 2": c2,
	}
}

func readMeta() []fs.FileMeta {
	result := []fs.FileMeta{}
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
