package cmd

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

type checkboxItem struct {
	Label       string
	Description string
	Checked     bool
}

// checkboxSelect renders an interactive checkbox list on the terminal
// (caller must ensure stdin is a TTY) and returns the indexes of the checked
// items. Keys: up/down (or k/j) move, space toggles, a toggles all, enter
// confirms, q or Ctrl-C aborts.
func checkboxSelect(out io.Writer, title string, items []checkboxItem) ([]int, error) {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer term.Restore(fd, old)        //nolint:errcheck // best effort on exit
	fmt.Fprint(out, "\x1b[?25l")       // hide cursor
	defer fmt.Fprint(out, "\x1b[?25h") // show cursor again

	cursor := 0
	draw := func(redraw bool) {
		if redraw {
			fmt.Fprintf(out, "\x1b[%dA", len(items)+1) // jump back to the title line
		}
		fmt.Fprintf(out, "\r\x1b[2K%s\r\n", title)
		for i, it := range items {
			prefix := "  "
			if i == cursor {
				prefix = "❯ "
			}
			box := "[ ]"
			if it.Checked {
				box = "[x]"
			}
			fmt.Fprintf(out, "\r\x1b[2K%s%s %-12s %s\r\n", prefix, box, it.Label, it.Description)
		}
	}
	draw(false)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		switch {
		case buf[0] == 3 || buf[0] == 'q': // Ctrl-C / q
			return nil, fmt.Errorf("selection aborted")
		case buf[0] == '\r' || buf[0] == '\n':
			var sel []int
			for i, it := range items {
				if it.Checked {
					sel = append(sel, i)
				}
			}
			return sel, nil
		case buf[0] == ' ':
			items[cursor].Checked = !items[cursor].Checked
		case buf[0] == 'a':
			all := true
			for _, it := range items {
				if !it.Checked {
					all = false
					break
				}
			}
			for i := range items {
				items[i].Checked = !all
			}
		case buf[0] == 'k', n == 3 && buf[1] == '[' && buf[2] == 'A': // up
			if cursor > 0 {
				cursor--
			}
		case buf[0] == 'j', n == 3 && buf[1] == '[' && buf[2] == 'B': // down
			if cursor < len(items)-1 {
				cursor++
			}
		}
		draw(true)
	}
}
