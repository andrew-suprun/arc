package app

import (
	"arc/fs"
	"arc/lifecycle"
)

func Run(roots []string, lc *lifecycle.Lifecycle, fsys fs.FS) {
	screen := initUi()
	defer deinitUi(screen)

	app := &appState{
		lc: lc,
		fs: fsys,
	}
	uiEvents := newUiEvents()

	go runUi(screen, uiEvents)

	for i, root := range roots {
		rootFolder := &file{
			folder: &folder{
				sortAscending: []bool{true, true, true},
			},
		}
		archive := &archive{
			idx:        i,
			rootPath:   root,
			rootFolder: rootFolder,
			curFolder:  rootFolder,
		}
		rootFolder.archive = archive
		app.archives = append(app.archives, archive)
		if i == 0 {
			app.curArchive = archive
		}

		fsys.Scan(root)
	}

	for !app.lc.ShoudStop() {
		select {
		case event := <-fsys.Events():
			app.handleFsEvent(event)
		case event := <-uiEvents:
			app.handleUiEvent(event)
		}
	loop:
		for {
			select {
			case event := <-fsys.Events():
				app.handleFsEvent(event)
			case event := <-uiEvents:
				app.handleUiEvent(event)
			default:
				break loop
			}
		}

		app.curArchive.rootFolder.updateMetas()
		app.sort()
		app.render(screen)
	}
}
