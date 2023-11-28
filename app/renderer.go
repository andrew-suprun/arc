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
	styleProgressBar    = tcell.StyleDefault.Foreground(tcell.Color231).Background(tcell.Color33).Bold(true)
)

func (app *appState) render(screen tcell.Screen) {
	folder := app.curArchive.curFolder

	b := &builder{width: width(app.screenWidth), height: app.screenHeight, screen: screen}

	if app.screenWidth < 80 || app.screenHeight < 24 {
		b.style(styleScreenTooSmall)
		for i := 0; i < app.screenHeight; i++ {
			if i == app.screenHeight/2 {
				b.text("", flex(1))
				b.text("Too Small...")
				b.text("", flex(1))
				b.newLine()
			} else {
				b.text("", flex(1))
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

	folder.getSelected()

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
	b.style(styleAppName)
	b.text(" Archive ")
	b.style(styleArchive)
	b.text(app.curArchive.rootPath, flex(1))
	b.newLine()
}

func (app *appState) breadcrumbs(b *builder) {
	app.folderTargets = app.folderTargets[:0]
	path := app.curArchive.curFolder.fullPath()

	b.style(styleBreadcrumbs)
	b.text(" Root", func(offset, width width) {
		app.folderTargets = append(app.folderTargets, folderTarget{
			path:   nil,
			offset: offset,
			width:  width,
		})
	})

	for i, name := range path {
		i := i
		b.style(styleDefault)
		b.text(" / ")
		b.style(styleBreadcrumbs)
		b.text(name, func(offset, width width) {
			app.folderTargets = append(app.folderTargets, folderTarget{
				path:   path[:i+1],
				offset: offset,
				width:  width,
			})
		})
	}

	b.text("", flex(1))
	b.newLine()
}

func (app *appState) folderView(b *builder) {
	folder := app.curArchive.curFolder
	app.sortTargets = make([]sortTarget, 3)

	b.style(styleFolderHeader)
	b.text(" State", width(11))
	b.text("   Document"+folder.sortIndicator(sortByName), width(23), flex(1), func(offset, width width) {
		app.sortTargets[0] = sortTarget{
			sortColumn: sortByName,
			offset:     offset,
			width:      width,
		}
	})

	b.text("   Date Modified"+folder.sortIndicator(sortByTime), width(22), func(offset, width width) {
		app.sortTargets[1] = sortTarget{
			sortColumn: sortByTime,
			offset:     offset,
			width:      width,
		}
	})

	b.text(fmt.Sprintf("%19s", "Size"+folder.sortIndicator(sortBySize)), func(offset, width width) {
		app.sortTargets[2] = sortTarget{
			sortColumn: sortBySize,
			offset:     offset,
			width:      width,
		}
	})
	b.text(" ")
	b.newLine()

	lines := app.screenHeight - 4

	for i := range folder.children[folder.offsetIdx:] {
		file := folder.children[folder.offsetIdx+i]
		if i >= lines {
			break
		}
		style := fileStyle(file)
		if folder.selectedIdx == folder.offsetIdx+i {
			style = style.Background(tcell.Color20)
		}
		b.style(style)
		app.state(b, file, width(11))
		if file.folder == nil {
			b.text("   ")
		} else {
			b.text(" ▶ ")
		}
		b.text(file.name, width(20), flex(1))
		b.text(file.modTime.Format(modTimeFormat))
		b.text(formatSize(file.size))
		b.text(" ")
		b.newLine()
	}
	b.style(styleDefault)
	rows := len(folder.children) - folder.offsetIdx
	for rows < lines {
		b.text("", flex(1))
		b.newLine()
		rows++
	}
}

func fileStyle(file *file) tcell.Style {
	if file.folder != nil && file.nHashed > 0 && file.nHashed < file.nFiles ||
		file.copied > 0 && file.copied < file.size {
		return tcell.StyleDefault.Foreground(tcell.PaletteColor(51)).Background(tcell.PaletteColor(17))
	}
	fg := 231
	switch file.state {
	case scanned:
		fg = 248
	case copying:
		fg = 51
	case pending:
		fg = 214
	case divergent:
		fg = 196
	}
	return tcell.StyleDefault.Foreground(tcell.PaletteColor(fg)).Background(tcell.PaletteColor(17))
}

func (app *appState) statusLine(b *builder) {

	root := app.curArchive.rootFolder
	var stage string
	var value float64
	if root.nHashed > 0 && root.nHashed < root.nFiles {
		stage = " Hashing"
		value = float64(root.nHashed) / float64(root.nFiles)
	} else if root.copied > 0 && root.copied < root.copying {
		stage = " Copying"
		value = float64(root.copied) / float64(root.copying)
	}
	b.style(styleArchive)
	if stage == "" {
		b.text(" All Clear", flex(1))
	} else {
		b.text(stage)
		b.text(fmt.Sprintf(" %6.2f%% ", value*100))
		b.progressBar(value, flex(1), styleProgressBar)
		b.text(" ")
	}

	b.newLine()
}

func (app *appState) state(b *builder, file *file, config config) {
	if app.curArchive.archiveState == archiveScanning {
		b.text("", config)
		return
	}
	if file.folder != nil && file.nHashed > 0 && file.nHashed < file.nFiles {
		value := float64(file.nHashed) / float64(file.nFiles)
		b.text(" ")
		b.progressBar(value, width(10), b.curStyle.Foreground(tcell.Color231).Background(tcell.Color33))
		return
	}
	if file.copied > 0 && file.copied < file.copying {
		value := float64(file.copied) / float64(file.copying)
		b.text(" ")
		b.progressBar(value, width(10), b.curStyle.Foreground(tcell.Color231).Background(tcell.Color33))
		return
	}
	showCounts := file.folder == nil && file.state == divergent
	if !showCounts {
		for _, count := range file.counts {
			if count != 1 {
				showCounts = true
				break
			}
		}
	}
	if showCounts {
		b.text(fileCounts(file), config)
		return
	}
	switch file.state {
	case scanned, hashed, copying:
		b.text("", config)

	case pending:
		b.text(" Pending", config)

	case duplicate:
		b.text(" Duplicates", config)

	case divergent:
		b.text(" Divergent", config)

	default:
		panic("invalid file state")
	}
}
