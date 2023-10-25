package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

type builder struct {
	width  int
	height int
	screen tcell.Screen
	sync   bool

	widths  []int
	offsets []int
	x, y    int
	field   int
}

type c struct {
	size, flex int
}

func (b *builder) pos(x, y int) {
	b.x = x
	b.y = y
}

func (b *builder) newLine() {
	b.x = 0
	b.y++
	b.field = 0
}

func (b *builder) layout(constraints ...c) {
	b.widths = make([]int, len(constraints))
	b.field = 0
	totalSize, totalFlex := 0, 0
	for i, cons := range constraints {
		b.widths[i] = cons.size
		totalSize += cons.size
		totalFlex += cons.flex
	}
	for totalSize > b.width {
		idx := 0
		maxSize := b.widths[0]
		for i, size := range b.widths {
			if maxSize < size {
				maxSize = size
				idx = i
			}
		}
		b.widths[idx]--
		totalSize--
	}

	if totalFlex == 0 {
		return
	}

	if totalSize < b.width {
		diff := b.width - totalSize
		remainders := make([]float64, len(constraints))
		for i, cons := range constraints {
			rate := float64(diff*cons.flex) / float64(totalFlex)
			remainders[i] = rate - math.Floor(rate)
			b.widths[i] += int(rate)
		}
		totalSize := 0
		for _, size := range b.widths {
			totalSize += size
		}
		for i := range b.widths {
			if totalSize == b.width {
				break
			}
			if constraints[i].flex > 0 {
				b.widths[i]++
				totalSize++
			}
		}
		for i := range b.widths {
			if totalSize == b.width {
				break
			}
			if constraints[i].flex == 0 {
				b.widths[i]++
				totalSize++
			}
		}
	}
	b.offsets = make([]int, len(b.widths))
	b.offsets[0] = 0
	for i := 1; i < len(b.offsets); i++ {
		b.offsets[i] = b.offsets[i-1] + b.widths[i-1]
	}
}

func (b *builder) text(text string, style tcell.Style) {
	runes := trim([]rune(text), b.widths[b.field])
	for i, ch := range runes {
		b.screen.SetContent(b.x+i, b.y, ch, nil, style)
	}
	b.x += len(runes)
	b.field++
}

const modTimeFormat = "  " + time.RFC3339

func (b *builder) state(file *file, style tcell.Style) {
	switch file.state {
	case scanned, hashed:
		b.text("", style)

	case inProgress:
		value := float64(file.progress) / float64(file.size)
		b.progressBar(value, styleProgressBar)

	case pending:
		b.text(" Pending", style)

	case divergent:
		b.text(" ---", style)
		// b.text(file.counts, style) // TODO
	}
}

func formatSize(size int) string {
	str := fmt.Sprintf("%15d", size)
	slice := []string{str[:3], str[3:6], str[6:9], str[9:12]}
	b := strings.Builder{}
	for _, s := range slice {
		b.WriteString(s)
		if s == " " || s == "   " {
			b.WriteString(" ")
		} else {
			b.WriteString(",")
		}
	}
	b.WriteString(str[12:])
	return b.String()
}

func (f *folder) sortIndicator(column sortColumn) string {
	if column == f.sortColumn {
		if f.sortAscending[column] {
			return " ▲"
		}
		return " ▼"
	}
	return ""
}

func (b *builder) progressBar(value float64, style tcell.Style) {
	if value < 0 || value > 1 {
		panic(fmt.Sprintf("Invalid progressBar value: %v", value))
	}
	width := b.widths[b.field]

	runes := make([]rune, width)
	progress := int(math.Round(float64(width*8) * float64(value)))
	idx := 0
	for ; idx < progress/8; idx++ {
		runes[idx] = '█'
	}
	if progress%8 > 0 {
		runes[idx] = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}[progress%8]
		idx++
	}
	for ; idx < int(width); idx++ {
		runes[idx] = ' '
	}
	runes = trim(runes, width)
	for i, ch := range trim(runes, width) {
		b.screen.SetContent(b.x+i, b.y, ch, nil, style)
	}
	b.field++
	b.x += len(runes)
}

func (b *builder) space(width, height int, style tcell.Style) {
	for line := b.y; line < b.y+height; line++ {
		for row := b.x; row < b.x+width; row++ {
			b.screen.SetContent(row, line, ' ', nil, style)
		}
	}
	b.y += height - 1
	b.x += width
	b.field++
}

func trim(runes []rune, width int) []rune {
	if width < 1 {
		return nil
	}
	if len(runes) > int(width) {
		runes = append(runes[:width-1], '…')
	}
	diff := int(width) - len(runes)
	for diff > 0 {
		runes = append(runes, ' ')
		diff--
	}
	return runes
}

func (b *builder) show() {
	if b.sync {
		b.screen.Sync()
	} else {
		b.screen.Show()
	}
}
