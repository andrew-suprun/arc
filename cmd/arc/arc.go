package main

import (
	"arc/app"
	"arc/fs"
	"arc/fs/filesys"
	"arc/fs/mockfs"
	"arc/lifecycle"
	"arc/log"
	"os"
)

func main() {
	log.SetLogger("log-arc.log")
	defer log.CloseLogger()

	var lc = lifecycle.New()
	var paths []string
	var fsys fs.FS
	if len(os.Args) > 1 && (os.Args[1] == "-sim" || os.Args[1] == "-sim2") {
		fsys = mockfs.NewFS(lc, os.Args[1] == "-sim")
		paths = []string{"origin", "copy 1", "copy 2"}
	} else {
		paths = make([]string, len(os.Args)-1)
		for i, path := range os.Args[1:] {
			err := os.MkdirAll(path, 0755)
			if err != nil {
				log.Debug("Failed to scan archives", "error", err)
				panic(err)
			}
			path, err := filesys.AbsPath(path)
			paths[i] = path
			if err != nil {
				log.Debug("Failed to scan archives", "error", err)
				panic(err)
			}
		}
		fsys = filesys.NewFS(lc)
	}

	app.Run(paths, lc, fsys)
}
