package filesys

import (
	"arc/fs"
	"arc/stream"
	"os"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/text/unicode/norm"
)

type fsys struct {
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

func NewFS() fs.FS {
	fs := &fsys{
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

func (fs *fsys) Quit() {
	fs.quit.Store(true)
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

func (fs *fsys) run() {
	for !fs.quit.Load() {
		commands, _ := fs.commands.Pull()
		for _, command := range commands {
			switch cmd := command.(type) {
			case scan:
				fs.scanArchive(cmd)
			case copy:
				fs.copyFile(cmd)
			case rename:
				fs.renameFile(cmd)
			case delete:
				fs.deleteFile(cmd)
			}
		}
	}
}

func (fs *fsys) scanArchive(scan scan) {
}

func (fs *fsys) copyFile(copy copy) {
}

func (fs *fsys) renameFile(rename rename) {
}

func (fs *fsys) deleteFile(delete delete) {
}
