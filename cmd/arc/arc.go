package main

import (
	"arc/app"
	"arc/fs"
	"arc/fs/filesys"
	"arc/fs/mockfs"
	"arc/log"
	"os"
)

func main() {
	log.SetLogger("log-arc.log")
	defer log.CloseLogger()

	var paths []string
	var fs fs.FS
	if len(os.Args) > 1 && (os.Args[1] == "-sim" || os.Args[1] == "-sim2") {
		fs = mockfs.NewFS(os.Args[1] == "-sim")
	} else {
		paths = make([]string, len(os.Args)-1)
		for i, path := range os.Args[1:] {
			path, err := filesys.AbsPath(path)
			paths[i] = path
			if err != nil {
				log.Debug("Failed to scan archives", "error", err)
				panic(err)
			}
		}
		fs = filesys.NewFS()
	}

	app.Run(paths, fs)
}
