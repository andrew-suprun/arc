package filesys

import (
	"arc/fs"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/text/unicode/norm"
)

func (f *fsys) copyFile(copy copy) {
	f.lc.Started()
	defer f.lc.Done()

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

	go f.reader(copy, events)

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
			f.events <- fs.CopyProgress{
				Root:   copy.fromRoot,
				Path:   copy.path,
				Copyed: reported,
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

func (f *fsys) reader(copy copy, eventChans []chan event) {
	commands := make([]chan []byte, len(copy.toRoots))
	defer func() {
		for _, cmdChan := range commands {
			close(cmdChan)
		}
	}()

	source := filepath.Join(copy.fromRoot, copy.path)
	info, err := os.Stat(source)
	if err != nil {
		f.events <- fs.Error{Path: copy.fromRoot, Error: err}
		return
	}

	for i, root := range copy.toRoots {
		commands[i] = make(chan []byte)
		go f.writer(root, copy.path, copy.hash, info.ModTime(), commands[i], eventChans[i])
	}

	sourceFile, err := os.Open(source)
	if err != nil {
		f.events <- fs.Error{Path: copy.fromRoot, Error: err}
		return
	}

	defer sourceFile.Close()

	var n int
	for err != io.EOF && !f.lc.ShoudStop() {
		buf := make([]byte, bufSize)
		n, err = sourceFile.Read(buf)
		if err != nil && err != io.EOF {
			f.events <- fs.Error{Path: copy.fromRoot, Error: err}
			return
		}
		for _, cmd := range commands {
			cmd <- buf[:n]
		}
	}
}

func (f *fsys) writer(root, path, hash string, modTime time.Time, cmdChan chan []byte, eventChan chan event) {
	var copied copyProgress

	fullPath := filepath.Join(root, path)
	dirPath := filepath.Dir(fullPath)
	_ = os.MkdirAll(dirPath, 0755)
	file, err := os.Create(fullPath)
	if err != nil {
		f.events <- fs.Error{Path: fullPath, Error: err}
		return
	}

	defer func() {
		if file != nil {
			info, _ := file.Stat()
			sys := info.Sys().(*syscall.Stat_t)
			size := info.Size()
			_ = file.Close()
			_ = os.Chtimes(fullPath, time.Now(), modTime)

			absHashFileName := filepath.Join(root, hashFileName)
			hashInfoFile, err := os.OpenFile(absHashFileName, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				csvWriter := csv.NewWriter(hashInfoFile)
				_ = csvWriter.Write([]string{
					fmt.Sprint(sys.Ino),
					norm.NFC.String(path),
					fmt.Sprint(size),
					modTime.UTC().Format(time.RFC3339Nano),
					hash,
				})
				csvWriter.Flush()
				_ = hashInfoFile.Close()
			}

			if f.lc.ShoudStop() {
				_ = os.Remove(dirPath)
			}
		}
		close(eventChan)
	}()

	for cmd := range cmdChan {
		if f.lc.ShoudStop() {
			// TODO: remove partly written file
			return
		}

		n, err := file.Write([]byte(cmd))
		copied += copyProgress(n)
		if err != nil {
			f.events <- fs.Error{Path: fullPath, Error: err}
			return
		}
		eventChan <- copied
	}
}
