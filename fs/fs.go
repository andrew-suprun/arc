package fs

import (
	"fmt"
	"time"
)

type (
	FS interface {
		Events() <-chan Event
		Scan(root string)
		Copy(path, hash, fromRoot string, toRoots ...string)
		Rename(root, sourcePath, targetPath string)
		Delete(path string)
		Quit()
	}

	Event interface {
		event()
	}

	FileMeta struct {
		Root    string
		Path    string
		Size    int
		ModTime time.Time
		Hash    string
	}

	FileHashed struct {
		Root string
		Path string
		Hash string
	}

	CopyProgress struct {
		Root   string
		Path   string
		Copyed int
	}

	ArchiveHashed struct {
		Root string
	}

	Copied struct {
		Path     string
		FromRoot string
		ToRoots  []string
	}

	Renamed struct {
		Root       string
		SourcePath string
		TargetPath string
	}

	Deleted struct {
		Path string
	}

	Error struct {
		Path  string
		Error error
	}
)

func (FileMeta) event()      {}
func (FileHashed) event()    {}
func (CopyProgress) event()  {}
func (ArchiveHashed) event() {}
func (Copied) event()        {}
func (Renamed) event()       {}
func (Deleted) event()       {}
func (Error) event()         {}

func (event FileMeta) String() string {
	return fmt.Sprintf("FileMeta{Root: %q, Path: %q, Size: %d, ModTime: %s}", event.Root, event.Path, event.Size, event.ModTime.Format("2006-01-02 15:04:05"))
}

func (event FileHashed) String() string {
	return fmt.Sprintf("FileHashed{Root: %q, Path: %q, Hash: %q}", event.Root, event.Path, event.Hash)
}
