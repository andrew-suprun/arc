package fs

import "time"

type (
	FS interface {
		Events() <-chan Event
		Scan(root string)
		Copy(path, fromRoot string, toRoots ...string)
		Rename(root, sourcePath, targetPath string)
		Delete(path string)
		Quit()
	}

	Event interface {
		event()
	}

	FileMetas []FileMeta

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

func (FileMetas) event()     {}
func (FileHashed) event()    {}
func (CopyProgress) event()  {}
func (ArchiveHashed) event() {}
func (Copied) event()        {}
func (Renamed) event()       {}
func (Deleted) event()       {}
func (Error) event()         {}
