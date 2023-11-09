package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

var (
	styleDefault        = tcell.StyleDefault.Foreground(tcell.Color250).Background(tcell.Color17)
	styleScreenTooSmall = tcell.StyleDefault.Foreground(tcell.Color231).Background(tcell.ColorRed).Bold(true)
	styleAppName        = tcell.StyleDefault.Foreground(tcell.Color226).Background(tcell.ColorBlack).Bold(true).Italic(true)
	styleArchive        = tcell.StyleDefault.Foreground(tcell.Color226).Background(tcell.ColorBlack).Bold(true)
	styleBreadcrumbs    = tcell.StyleDefault.Foreground(tcell.Color231).Background(tcell.Color17).Bold(true).Italic(true)
	styleFolderHeader   = tcell.StyleDefault.Foreground(tcell.Color231).Background(tcell.ColorGray).Bold(true)
	styleProgressBar    = tcell.StyleDefault.Foreground(tcell.Color231).Background(tcell.ColorLightGray)
)

func (app *appState) render(screen tcell.Screen) {
	folder := app.curArchive.curFolder
	folder.sort()
	folder.updateMetas()

	b := &builder{width: app.screenWidth, height: app.screenHeight, screen: screen}

	if app.screenWidth < 80 || app.screenHeight < 24 {
		for i := 0; i < app.screenHeight; i++ {
			if i == app.screenHeight/2 {
				b.text("", config{style: styleScreenTooSmall, flex: 1})
				b.text("Too Small...", config{style: styleScreenTooSmall})
				b.text("", config{style: styleScreenTooSmall, flex: 1})
				b.newLine()
			} else {
				b.text("", config{style: styleScreenTooSmall, flex: 1})
				b.newLine()
			}
		}
		b.show(app.sync)
		app.sync = false
		return
	}

	lines := app.screenHeight - 4
	entries := len(folder.children)
	if folder.offsetIdx >= entries-lines+1 {
		folder.offsetIdx = entries + 1 - lines
	}
	if folder.offsetIdx < 0 {
		folder.offsetIdx = 0
	}
	if folder.selectedIdx >= entries {
		folder.selectedIdx = entries - 1
	}
	if folder.selectedIdx < 0 {
		folder.selectedIdx = 0
	}
	if app.makeSelectedVisible {
		if folder.offsetIdx <= folder.selectedIdx-lines {
			folder.offsetIdx = folder.selectedIdx + 1 - lines
		}
		if folder.offsetIdx > folder.selectedIdx {
			folder.offsetIdx = folder.selectedIdx
		}
		app.makeSelectedVisible = false
	}

	app.showTitle(b)
	app.breadcrumbs(b)
	app.folderView(b)
	app.statusLine(b)

	b.show(app.sync)
	app.sync = false
}

func (app *appState) showTitle(b *builder) {
	b.text(" Archive ", config{style: styleAppName, width: 9})
	b.text(app.curArchive.rootPath, config{style: styleArchive, flex: 1})
	b.newLine()
}

func (app *appState) breadcrumbs(b *builder) {
	app.folderTargets = app.folderTargets[:0]
	path := app.curArchive.curFolder.fullPath()

	b.text(" Root", config{style: styleBreadcrumbs, handler: func(offset, width int) {
		app.folderTargets = append(app.folderTargets, folderTarget{
			path:   nil,
			offset: offset,
			width:  width,
		})
	}})

	for i, name := range path {
		i := i
		b.text(" / ", config{style: styleDefault})
		b.text(name, config{style: styleBreadcrumbs, handler: func(offset, width int) {
			app.folderTargets = append(app.folderTargets, folderTarget{
				path:   path[:i+1],
				offset: offset,
				width:  width,
			})
		}})
	}

	b.text("", config{style: styleBreadcrumbs, flex: 1})
	b.newLine()
}

func (app *appState) folderView(b *builder) {
	folder := app.curArchive.curFolder
	app.sortTargets = make([]sortTarget, 3)

	b.text(" State", config{style: styleFolderHeader, width: 11})
	b.text("   Document"+folder.sortIndicator(sortByName), config{
		style: styleFolderHeader,
		width: 23,
		flex:  1,
		handler: func(offset, width int) {
			app.sortTargets[0] = sortTarget{
				sortColumn: sortByName,
				offset:     offset,
				width:      width,
			}
		},
	})

	b.text("  Date Modified"+folder.sortIndicator(sortByTime), config{
		style: styleFolderHeader,
		width: 22,
		handler: func(offset, width int) {
			app.sortTargets[1] = sortTarget{
				sortColumn: sortByTime,
				offset:     offset,
				width:      width,
			}
		},
	})

	b.text(fmt.Sprintf("%19s", "Size"+folder.sortIndicator(sortBySize)), config{
		style: styleFolderHeader,
		handler: func(offset, width int) {
			app.sortTargets[2] = sortTarget{
				sortColumn: sortBySize,
				offset:     offset,
				width:      width,
			}
		},
	})
	b.text(" ", config{style: styleFolderHeader})
	b.newLine()

	lines := app.screenHeight - 4

	for i := range folder.children[folder.offsetIdx:] {
		file := folder.children[folder.offsetIdx+i]
		if i >= lines {
			break
		}

		style := fileStyle(file).Reverse(folder.selectedIdx == folder.offsetIdx+i)
		b.state(file, config{style: style, width: 11})
		if file.folder == nil {
			b.text("   ", config{style: style})
		} else {
			b.text(" ▶ ", config{style: style})
		}
		b.text(file.name, config{style: style, width: 20, flex: 1})
		b.text(file.modTime.Format(modTimeFormat), config{style: style})
		b.text(formatSize(file.size), config{style: style})
		b.text(" ", config{style: style})
		b.newLine()
	}
	rows := len(folder.children) - folder.offsetIdx
	for rows < lines {
		b.text("", config{style: styleDefault, flex: 1})
		b.newLine()
		rows++
	}
}

func fileStyle(file *file) tcell.Style {
	fg := 231
	switch file.state {
	case scanned:
		fg = 248
	case inProgress:
		fg = 195
	case pending:
		fg = 214
	case divergent:
		fg = 196
	}
	return tcell.StyleDefault.Foreground(tcell.PaletteColor(fg)).Background(tcell.PaletteColor(17))
}

func (app *appState) statusLine(b *builder) {
	b.text(" Status line will be here...", config{style: styleArchive, flex: 1})
	b.newLine()
}
