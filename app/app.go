package app

import (
	"arc/fs"
)

func Run(roots []string, fsys fs.FS) {
	screen := initUi()
	defer deinitUi(screen)

	app := &appState{
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
		arc := &archive{
			rootPath:   root,
			rootFolder: rootFolder,
			curFolder:  rootFolder,
		}
		app.archives = append(app.archives, arc)
		if i == 0 {
			app.curArchive = arc
		}

		fsys.Scan(root)
	}

	for {
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
		if app.quit {
			break
		}
		app.render(screen)
	}
}
