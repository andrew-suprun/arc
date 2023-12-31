package filesys

import (
	"arc/fs"
	"arc/lifecycle"
	"arc/log"
	"arc/stream"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

type fsys struct {
	commands *stream.Stream[command]
	events   chan fs.Event
	lc       *lifecycle.Lifecycle
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

const bufSize = 256 * 1024

func NewFS(lc *lifecycle.Lifecycle) fs.FS {
	fs := &fsys{
		commands: stream.NewStream[command]("commands"),
		events:   make(chan fs.Event, 256),
		lc:       lc,
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

func (fs *fsys) Quit() {
	fs.commands.Close()
	fs.lc.Stop()
}

func AbsPath(path string) (string, error) {
	var err error
	path, err = filepath.Abs(path)
	path = norm.NFC.String(path)
	if err != nil {
		return "", err
	}

	_, err = os.Stat(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (f *fsys) run() {
	for {
		for _, command := range f.commands.Pull() {
			if f.lc.ShoudStop() {
				return
			}
			switch cmd := command.(type) {
			case scan:
				go f.scanArchive(cmd)
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

func (f *fsys) renameFile(rename rename) {
	log.Debug("rename", "root", rename.root, "source", rename.sourcePath, "target", rename.targetPath)
	path := filepath.Join(rename.root, filepath.Dir(rename.targetPath))
	err := os.MkdirAll(path, 0755)
	if err != nil {
		f.events <- fs.Error{Path: path, Error: err}
		return
	}
	from := filepath.Join(rename.root, rename.sourcePath)
	to := filepath.Join(rename.root, rename.targetPath)
	err = os.Rename(from, to)
	if err != nil {
		f.events <- fs.Error{Path: to, Error: err}
		return
	}
	f.events <- fs.Renamed{
		Root:       rename.root,
		SourcePath: rename.sourcePath,
		TargetPath: rename.targetPath,
	}
	f.removeDirIfEmpty(filepath.Dir(from))
}

func (f *fsys) deleteFile(delete delete) {
	log.Debug("delete", "path", delete.path)
	err := os.Remove(delete.path)
	if err != nil {
		f.events <- fs.Error{Path: delete.path, Error: err}
		return
	}
	f.events <- fs.Deleted{Path: delete.path}
	f.removeDirIfEmpty(filepath.Dir(delete.path))
}

func (f *fsys) removeDirIfEmpty(path string) {
	fsys := os.DirFS(path)

	entries, _ := iofs.ReadDir(fsys, ".")
	hasFiles := false
	for _, entry := range entries {
		if entry.Name() != ".DS_Store" && !strings.HasPrefix(entry.Name(), "._") {
			info, err := entry.Info()
			if err != nil {
				f.events <- fs.Error{Path: path, Error: err}
				return
			}

			size := int(info.Size())
			if size > 0 {
				hasFiles = true
				break
			}
		}
	}
	if !hasFiles {
		os.RemoveAll(path)
	}
}
