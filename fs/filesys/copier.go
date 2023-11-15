package filesys

import (
	"arc/fs"
	"arc/log"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (f *fsys) copyFile(copy copy) {
	log.Debug("copy", "from", copy.fromRoot)
	for _, to := range copy.toRoots {
		log.Debug("copy", "to", to)
	}

	defer func() {
		f.events <- fs.Copied{
			Path:     copy.path,
			FromRoot: copy.fromRoot,
			ToRoots:  copy.toRoots,
		}
	}()

	events := make([]chan event, len(copy.toRoots))
	copied := make([]int, len(copy.toRoots))
	reported := 0

	for i := range copy.toRoots {
		events[i] = make(chan event, 1)
	}

	go f.reader(copy.fromRoot, copy.toRoots, events)

	for {
		hasValue := false
		minCopied := 0
		for i := range events {
			if event, ok := <-events[i]; ok {
				hasValue = true
				switch event := event.(type) {
				case copyProgress:
					copied[i] = int(event)
					minCopied = copied[i]

				case copyError:
					f.events <- fs.Error{Path: event.Path, Error: event.Error}
				}
			}
		}
		for _, fileCopied := range copied {
			if minCopied > fileCopied {
				minCopied = fileCopied
			}
		}
		if reported < minCopied {
			reported = minCopied
			f.events <- fs.Progress{
				Root:     copy.fromRoot,
				Path:     copy.path,
				Progress: reported,
			}
		}
		if !hasValue {
			break
		}
	}
}

type event interface {
	event()
}

type copyProgress int

func (copyProgress) event() {}

type copyError fs.Error

func (copyError) event() {}

func (f *fsys) reader(source string, targets []string, eventChans []chan event) {
	commands := make([]chan []byte, len(targets))
	defer func() {
		for _, cmdChan := range commands {
			close(cmdChan)
		}
	}()

	info, err := os.Stat(source)
	if err != nil {
		f.events <- fs.Error{Path: source, Error: err}
		return
	}

	for i, to := range targets {
		commands[i] = make(chan []byte)
		go f.writer(to, info.ModTime(), commands[i], eventChans[i])
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		f.events <- fs.Error{Path: source, Error: err}
		return
	}

	var n int
	for err != io.EOF && !f.quit.Load() {
		buf := make([]byte, 1024*1024)
		n, err = sourceFile.Read(buf)
		if err != nil && err != io.EOF {
			f.events <- fs.Error{Path: source, Error: err}
			return
		}
		for _, cmd := range commands {
			cmd <- buf[:n]
		}
	}
}

func (f *fsys) writer(to string, modTime time.Time, cmdChan chan []byte, eventChan chan event) {
	var copied copyProgress

	filePath := filepath.Dir(to)
	os.MkdirAll(filePath, 0755)
	file, err := os.Create(to)
	if err != nil {
		f.events <- fs.Error{Path: to, Error: err}
		return
	}

	defer func() {
		if file != nil {
			file.Close()
			if f.quit.Load() {
				os.Remove(filePath)
			}
			os.Chtimes(to, time.Now(), modTime)
		}
		close(eventChan)
	}()

	for cmd := range cmdChan {
		if f.quit.Load() {
			return
		}

		n, err := file.Write([]byte(cmd))
		copied += copyProgress(n)
		if err != nil {
			f.events <- fs.Error{Path: to, Error: err}
			return
		}
		eventChan <- copied
	}
}
