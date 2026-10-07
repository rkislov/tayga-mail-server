package climenu

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type UI struct {
	in  *bufio.Reader
	out *os.File
}

func NewUI() *UI {
	return &UI{in: bufio.NewReader(os.Stdin), out: os.Stdout}
}

func (u *UI) Println(a ...any) {
	fmt.Fprintln(u.out, a...)
}

func (u *UI) Printf(format string, a ...any) {
	fmt.Fprintf(u.out, format, a...)
}

func (u *UI) Prompt(label, def string) string {
	if def != "" {
		u.Printf("%s [%s]: ", label, def)
	} else {
		u.Printf("%s: ", label)
	}
	line, err := u.in.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func (u *UI) PromptYesNo(label string, defYes bool) bool {
	def := "n"
	if defYes {
		def = "y"
	}
	for {
		ans := strings.ToLower(u.Prompt(label+" (y/n)", def))
		switch ans {
		case "y", "yes", "д", "да":
			return true
		case "n", "no", "н", "нет":
			return false
		}
	}
}

func (u *UI) Menu(title string, items []string) int {
	u.Println()
	u.Println("=== " + title + " ===")
	for i, it := range items {
		u.Printf("  %d) %s\n", i+1, it)
	}
	u.Printf("  0) Назад / выход\n")
	for {
		raw := u.Prompt("Выбор", "")
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		if n == 0 {
			return 0
		}
		if n >= 1 && n <= len(items) {
			return n
		}
	}
}
