package app

import (
	"fmt"
	"math"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type builder struct {
	width  int
	height int
	screen tcell.Screen
	line   int
	fields []*field
}

type config struct {
	style   tcell.Style
	width   int
	flex    int
	handler func(offset, width int)
}

type field struct {
	renderer
	style   tcell.Style
	width   int
	flex    int
	handler func(offset, width int)
}

func (b *builder) text(txt string, config config) {
	runes := []rune(txt)
	width := config.width
	if width == 0 {
		width = len(runes)
	}

	b.fields = append(b.fields, &field{
		renderer: &text{text: runes},
		style:    config.style,
		width:    width,
		flex:     config.flex,
		handler:  config.handler,
	})
}

func (b *builder) progressBar(value float64, style tcell.Style) {
	b.fields = append(b.fields, &field{
		renderer: &progressBar{value: value},
		style:    style,
		width:    10,
		flex:     1,
	})
}

func (b *builder) state(file *file, config config) {
	if file.progress > 0 && file.progress < file.size {
		value := float64(file.progress) / float64(file.size)
		b.progressBar(value, styleProgressBar)
		return
	}
	switch file.state {
	case scanned, hashed:
		b.text("", config)

	case pending:
		b.text(" Pending", config)

	case divergent:
		b.text(fileCounts(file), config)
	}
}

func (b *builder) layout() {
	totalWidth, totalFlex := 0, 0
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
			rate := float64(diff*field.flex) / float64(totalFlex)
			remainders[i] = rate - math.Floor(rate)
			b.fields[i].width += int(rate)
		}
		totalWidth := 0
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
	x := 0
	for _, field := range b.fields {
		if field.handler != nil {
			field.handler(x, field.width)
		}
		for i, ch := range field.runes(field.width) {
			b.screen.SetContent(x+i, b.line, ch, nil, field.style)
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
	runes(width int) []rune
}

type text struct {
	text []rune
}

func (t *text) runes(width int) []rune {
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

func (pb *progressBar) runes(width int) []rune {
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
