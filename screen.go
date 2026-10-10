package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	ansiReset      = "\x1b[0m"
	ansiClear      = "\x1b[2J\x1b[H"
	ansiCyan       = "\x1b[1;36m"
	ansiYellow     = "\x1b[1;33m"
	ansiGreen      = "\x1b[1;32m"
	ansiWhite      = "\x1b[1;37m"
	ansiRed        = "\x1b[1;31m"
	ansiDim        = "\x1b[0;36m"
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
	thermoWidth    = 50
)

func renderScreen(user, url string, files []CopiedFile, copyErrs []error) []byte {
	var b strings.Builder
	b.WriteString(ansiShowCursor)
	b.WriteString(ansiReset)
	b.WriteString(ansiClear)
	b.WriteString(ansiCyan)
	b.WriteString("  +--------------------------------------------------------------+\r\n")
	b.WriteString("  |  ")
	b.WriteString(ansiYellow)
	b.WriteString("SHS BBS  Web Download")
	b.WriteString(ansiCyan)
	b.WriteString("                                       |\r\n")
	b.WriteString("  +--------------------------------------------------------------+\r\n")
	b.WriteString(ansiReset)
	fmt.Fprintf(&b, "  %sUser:%s  %s%s\r\n\r\n", ansiDim, ansiReset, ansiWhite, user)
	b.WriteString(ansiGreen)
	b.WriteString("  Your files are ready on the web. Open this URL:\r\n\r\n")
	b.WriteString(ansiYellow)
	fmt.Fprintf(&b, "  %s\r\n\r\n", url)
	b.WriteString(ansiWhite)
	b.WriteString("  Files:\r\n")
	if len(files) == 0 {
		b.WriteString(ansiRed)
		b.WriteString("    (none copied)\r\n")
	} else {
		b.WriteString(ansiGreen)
		for _, f := range files {
			fmt.Fprintf(&b, "    %-32s  %s\r\n", f.Name, formatSize(f.Size))
		}
	}
	if len(copyErrs) > 0 {
		b.WriteString("\r\n")
		b.WriteString(ansiRed)
		b.WriteString("  Problems:\r\n")
		for _, err := range copyErrs {
			fmt.Fprintf(&b, "    %v\r\n", err)
		}
	}
	b.WriteString("\r\n")
	b.WriteString(ansiCyan)
	b.WriteString("  Download from the URL above.\r\n")
	b.WriteString("  Press ESCAPE or Ctrl-X when you are finished downloading.\r\n")
	b.WriteString(ansiReset)
	b.WriteString("\r\n")
	return []byte(b.String())
}

func renderWorking() []byte {
	return renderProgress(copyProgress{})
}

func renderProgress(p copyProgress) []byte {
	var b strings.Builder
	b.WriteString(ansiHideCursor)
	b.WriteString(ansiReset)
	b.WriteString(ansiClear)
	b.WriteString(ansiCyan)
	b.WriteString("  +--------------------------------------------------------------+\r\n")
	b.WriteString("  |  ")
	b.WriteString(ansiYellow)
	b.WriteString("SHS BBS  Web Download")
	b.WriteString(ansiCyan)
	b.WriteString("                                       |\r\n")
	b.WriteString("  +--------------------------------------------------------------+\r\n")
	b.WriteString(ansiReset)
	b.WriteString("\r\n")
	b.WriteString(ansiWhite)
	if p.FileCount > 0 {
		fmt.Fprintf(&b, "  Copying file %d of %d\r\n", p.FileNum, p.FileCount)
	} else {
		b.WriteString("  Copying files...\r\n")
	}
	b.WriteString(ansiYellow)
	name := p.Name
	if name == "" {
		name = "..."
	}
	fmt.Fprintf(&b, "  %s\r\n\r\n", fitName(name, 60))
	pct := percent(p.AllDone, p.AllTotal)
	b.WriteString("  ")
	b.WriteString(ansiCyan)
	b.WriteString("[")
	b.WriteString(ansiGreen)
	b.WriteString(thermometerFill(thermoWidth, p.AllDone, p.AllTotal))
	b.WriteString(ansiCyan)
	b.WriteString("]")
	b.WriteString(ansiWhite)
	fmt.Fprintf(&b, "  %3d%%\r\n", pct)
	b.WriteString(ansiDim)
	if p.AllTotal > 0 {
		fmt.Fprintf(&b, "  %s / %s\r\n", formatSize(p.AllDone), formatSize(p.AllTotal))
	} else {
		b.WriteString("\r\n")
	}
	b.WriteString(ansiReset)
	return []byte(b.String())
}

func thermometerFill(width int, done, total int64) string {
	if width < 1 {
		width = 1
	}
	fill := 0
	if total > 0 {
		fill = int((done*int64(width) + total/2) / total)
	} else if done > 0 {
		fill = width
	}
	if fill < 0 {
		fill = 0
	}
	if fill > width {
		fill = width
	}
	return strings.Repeat("#", fill) + strings.Repeat("-", width-fill)
}

func percent(done, total int64) int {
	if total <= 0 {
		if done <= 0 {
			return 0
		}
		return 100
	}
	p := int((done*100 + total/2) / total)
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func fitName(name string, n int) string {
	if n < 1 {
		return ""
	}
	if len(name) <= n {
		return name
	}
	if n <= 3 {
		return name[:n]
	}
	return name[:n-3] + "..."
}

type progressMeter struct {
	w     io.Writer
	last  time.Time
	lastP int
	once  bool
}

func (m *progressMeter) Update(p copyProgress) {
	if m == nil || m.w == nil {
		return
	}
	pct := percent(p.AllDone, p.AllTotal)
	now := time.Now()
	fileDone := p.FileSize > 0 && p.FileDone >= p.FileSize
	if m.once && !fileDone && pct == m.lastP && now.Sub(m.last) < 80*time.Millisecond {
		return
	}
	m.once = true
	m.last = now
	m.lastP = pct
	_, _ = m.w.Write(renderProgress(p))
}
