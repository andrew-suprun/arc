package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type builder struct {
	width    width
	height   int
	screen   tcell.Screen
	line     int
	fields   []*field
	curStyle tcell.Style
}

type config interface{}
type width int
type flex int
type handler func(offset, width width)

type field struct {
	renderer
	width
	flex
	handler
	style *tcell.Style
}

func (b *builder) style(style tcell.Style) {
	b.curStyle = style
}

func (b *builder) text(txt string, configs ...config) {
	runes := []rune(txt)
	field := &field{
		renderer: &text{text: runes},
	}
	field.config(configs)
	if field.width == 0 {
		field.width = width(len(runes))
	}
	b.fields = append(b.fields, field)
}

func (f *field) config(configs []config) {
	for _, config := range configs {
		switch config := config.(type) {
		case width:
			f.width = config
		case flex:
			f.flex = config
		case tcell.Style:
			f.style = &config
		case func(offset, width width):
			f.handler = config
		}
	}
}

func (b *builder) progressBar(value float64, configs ...config) {
	field := &field{
		renderer: &progressBar{value: value},
	}
	field.config(configs)
	b.fields = append(b.fields, field)

}

func (b *builder) state(file *file, config config) {
	if file.folder != nil && file.nHashed > 0 && file.nHashed < file.nFiles {
		value := float64(file.nHashed) / float64(file.nFiles)
		b.text(" ")
		b.progressBar(value, width(10), b.curStyle.Foreground(tcell.Color231).Background(tcell.Color33))
		return
	}
	if file.copied > 0 && file.copied < file.size {
		value := float64(file.copied) / float64(file.size)
		b.text(" ")
		b.progressBar(value, width(10), b.curStyle.Foreground(tcell.Color231).Background(tcell.Color33))
		return
	}
	showCounts := false
	for _, count := range file.counts {
		if count != 1 {
			showCounts = true
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

func (b *builder) layout() {
	totalWidth, totalFlex := width(0), flex(0)
	for _, field := range b.fields {
		totalWidth += field.width
		totalFlex += field.flex
	}
	for totalWidth > b.width {
		idx := 0
		maxSize := b.fields[0].width
		for i, field := range b.fields {
			if maxSize < field.width {
				maxSize = field.width
				idx = i
			}
		}
		b.fields[idx].width--
		totalWidth--
	}

	if totalFlex == 0 {
		return
	}

	if totalWidth < b.width {
		diff := b.width - totalWidth
		remainders := make([]float64, len(b.fields))
		for i, field := range b.fields {
			rate := float64(diff*width(field.flex)) / float64(totalFlex)
			remainders[i] = rate - math.Floor(rate)
			b.fields[i].width += width(rate)
		}
		totalWidth := width(0)
		for _, field := range b.fields {
			totalWidth += field.width
		}
		for _, field := range b.fields {
			if totalWidth == b.width {
				break
			}
			if field.flex > 0 {
				field.width++
				totalWidth++
			}
		}
		for _, field := range b.fields {
			if totalWidth == b.width {
				break
			}
			if field.flex == 0 {
				field.width++
				totalWidth++
			}
		}
	}
}

func (b *builder) newLine() {
	b.layout()
	x := width(0)
	for _, field := range b.fields {
		if field.handler != nil {
			field.handler(x, field.width)
		}
		style := b.curStyle
		if field.style != nil {
			style = *field.style
		}
		for i, ch := range field.runes(field.width) {
			b.screen.SetContent(int(x)+i, b.line, ch, nil, style)
		}
		x += field.width
	}
	b.fields = b.fields[:0]
	b.line++
}

func (b *builder) show(sync bool) {
	if sync {
		b.screen.Sync()
	} else {
		b.screen.Show()
	}
}

type renderer interface {
	runes(width width) []rune
}

type text struct {
	text []rune
}

func (t *text) runes(width width) []rune {
	if width < 1 {
		return nil
	}
	if len(t.text) > int(width) {
		t.text = append(t.text[:width-1], '…')
	}
	diff := int(width) - len(t.text)
	for diff > 0 {
		t.text = append(t.text, ' ')
		diff--
	}
	return t.text
}

type progressBar struct {
	value float64
}

func (pb *progressBar) runes(width width) []rune {
	if pb.value < 0 || pb.value > 1 {
		panic(fmt.Sprintf("Invalid progressBar value: %v", pb.value))
	}

	runes := make([]rune, width)
	progress := int(math.Round(float64(width*8) * float64(pb.value)))
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
	return runes
}

const modTimeFormat = "  2006-01-02T15:04:05"

func fileCounts(file *file) string {
	buf := &strings.Builder{}
	buf.WriteRune(' ')
	for _, count := range file.counts {
		fmt.Fprintf(buf, "%c", countRune(count))
	}
	return buf.String()
}

func countRune(count int) rune {
	if count == 0 {
		return '-'
	}
	if count > 9 {
		return '*'
	}
	return '0' + rune(count)
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
