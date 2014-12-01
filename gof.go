package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"code.google.com/p/mahonia"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

var duration = 10 * time.Millisecond

var edit = flag.Bool("e", false, "Edit selected file")
var cat = flag.Bool("c", false, "Cat the file")
var remove = flag.Bool("r", false, "Remove the file")
var launcher = flag.Bool("l", false, "Launcher mode")
var root = flag.String("d", "", "Root directory")
var multi = flag.Bool("m", false, "Multiple select")

func print_tb(x, y int, fg, bg termbox.Attribute, msg string) {
	for _, c := range []rune(msg) {
		termbox.SetCell(x, y, c, fg, bg)
		x += runewidth.RuneWidth(c)
	}
}

func printf_tb(x, y int, fg, bg termbox.Attribute, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	print_tb(x, y, fg, bg, s)
}

func edit_file(files []string) error {
	env := os.Getenv("GOFEDITOR")
	if env == "" {
		env = os.Getenv("EDITOR")
	}
	if env == "" {
		env = "vim"
	}
	args := strings.Split(env, " ")
	args = append(args, files...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

type matched struct {
	name string
	pos1 int
	pos2 int
	selected bool
}

type filtered []matched

func (f filtered) Less(i, j int) bool {
	return (f[i].pos2 - f[i].pos1) < (f[j].pos2 - f[j].pos1)
}

func (f filtered) Len() int {
	return len(f)
}

func (f filtered) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

var input = []rune{}
var files = []string{}
var selected = []string{}
var heading = false
var current filtered
var cursor_x, cursor_y int
var width, height int
var mutex sync.Mutex
var launcherFiles = []string{}

func filter() {
	mutex.Lock()
	defer mutex.Unlock()

	defer func() {
		recover()
	}()

	if len(input) == 0 {
		current = make(filtered, len(files))
		for n, f := range files[0:len(current)] {
			prev_selected := false
			for _, s := range selected {
				if f == s {
					prev_selected = true
					break
				}
			}
			current[n] = matched{
				name: f,
				pos1: -1,
				pos2: -1,
				selected: prev_selected,
			}
		}
	} else {
		pat := "(?i)(?:.*)("
		for _, r := range []rune(input) {
			pat += regexp.QuoteMeta(string(r)) + ".*?"
		}
		pat += ")"
		re := regexp.MustCompile(pat)

		current = make(filtered, 0, len(files))
		for _, f := range files {
			ms := re.FindAllStringSubmatchIndex(f, 1)
			if len(ms) != 1 || len(ms[0]) != 4 {
				continue
			}
			prev_selected := false
			for _, s := range selected {
				if f == s {
					prev_selected = true
					break
				}
			}
			current = append(current, matched{
				name: f,
				pos1: len([]rune(f[0:ms[0][2]])),
				pos2: len([]rune(f[0:ms[0][3]])),
				selected: prev_selected,
			})
		}
	}
	if len(input) > 0 {
		sort.Sort(current)
	}
	if cursor_y < 0 {
		cursor_y = 0
	}
	if cursor_y >= len(current) {
		cursor_y = len(current) - 1
	}
}

var scanning = 0
var drawing = false

func draw_screen() {
	mutex.Lock()
	defer mutex.Unlock()

	defer func() {
		recover()
	}()

	width, height = termbox.Size()
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	pat := ""
	for _, r := range input {
		pat += regexp.QuoteMeta(string(r)) + ".*?"
	}
	for n := 0; n < height-3; n++ {
		if n >= len(current) {
			break
		}
		x := 2
		w := 0
		name := current[n].name
		pos1 := current[n].pos1
		pos2 := current[n].pos2
		selected := current[n].selected
		if pos1 >= 0 {
			pwidth := runewidth.StringWidth(string([]rune(current[n].name)[0:pos1]))
			if !heading && pwidth > width/2 {
				rname := []rune(name)
				wwidth := 0
				for i := 0; i < len(rname); i++ {
					w = runewidth.RuneWidth(rname[i])
					if wwidth+w > width/2 {
						name = "..." + string(rname[i:])
						pos1 -= i - 3
						pos2 -= i - 3
						break
					}
					wwidth += w
				}
			}
		}
		swidth := runewidth.StringWidth(name)
		if swidth+2 > width {
			rname := []rune(name)
			name = string(rname[0:width-5]) + "..."
		}
		for f, c := range []rune(name) {
			w = runewidth.RuneWidth(c)
			if x+w > width {
				break
			}
			if pos1 <= f && f < pos2 {
				if selected {
					termbox.SetCell(x, height-4-n, c, termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault)
				} else if cursor_y == n {
					termbox.SetCell(x, height-4-n, c, termbox.ColorYellow|termbox.AttrBold|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, height-4-n, c, termbox.ColorGreen|termbox.AttrBold, termbox.ColorDefault)
				}
			} else {
				if selected {
					termbox.SetCell(x, height-4-n, c, termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault)
				} else if cursor_y == n {
					termbox.SetCell(x, height-4-n, c, termbox.ColorYellow|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, height-4-n, c, termbox.ColorWhite|termbox.AttrBold, termbox.ColorDefault)
				}
			}
			x += w
		}
	}
	if cursor_y >= 0 {
		print_tb(0, height-4-cursor_y, termbox.ColorRed|termbox.AttrBold, termbox.ColorBlack, "> ")
	}
	if scanning >= 0 {
		print_tb(0, height-3, termbox.ColorGreen|termbox.AttrBold, termbox.ColorBlack, string([]rune("-\\|/")[scanning%4]))
		scanning++
	}
	printf_tb(2, height-3, termbox.ColorWhite|termbox.AttrBold, termbox.ColorBlack, "%d/%d(%d)", len(current), len(files), len(selected))
	print_tb(0, height-2, termbox.ColorBlue|termbox.AttrBold, termbox.ColorBlack, "> ")
	print_tb(2, height-2, termbox.ColorWhite|termbox.AttrBold, termbox.ColorBlack, string(input))
	termbox.SetCursor(2+runewidth.StringWidth(string(input[0:cursor_x])), height-2)
	termbox.Flush()
}

func main() {
	flag.Parse()

	var err error
	cwd := ""

	if flag.NFlag() == 0 && flag.NArg() == 1 {
		*root = flag.Arg(0)
	}

	if *root == "" {
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		if runtime.GOOS == "windows" && strings.HasPrefix(*root, "/") {
			cwd, _ = os.Getwd()
			cwd = filepath.Join(filepath.VolumeName(cwd), *root)
		} else {
			cwd, err = filepath.Abs(*root)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		st, err := os.Stat(cwd)
		if err == nil && !st.IsDir() {
			err = fmt.Errorf("Directory not found: %s", cwd)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		err = os.Chdir(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	dirty := false
	terminating := false
	var quit chan bool

	timer := time.AfterFunc(0, func() {
		if dirty {
			filter()
			draw_screen()
			dirty = false
		} else {
			draw_screen()
		}
	})
	timer.Stop()

	is_tty := isatty()

	if !is_tty {
		var buf *bufio.Reader
		if enc := os.Getenv("GOFSTDINENC"); enc != "" {
			buf = bufio.NewReader(mahonia.NewDecoder(enc).NewReader(os.Stdin))
		} else {
			buf = bufio.NewReader(os.Stdin)
		}
		for {
			b, _, err := buf.ReadLine()
			if err != nil {
				break
			}
			mutex.Lock()
			files = append(files, string(b))
			mutex.Unlock()
			dirty = true
			timer.Reset(duration)
		}
		err = tty_ready()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if *launcher {
		home := os.Getenv("HOME")
		if home == "" && runtime.GOOS == "windows" {
			home = os.Getenv("USERPROFILE")
		}
		b, err := ioutil.ReadFile(filepath.Join(home, ".gof-launcher"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		launcherFiles = strings.Split(string(b), "\n")
		for _, line := range launcherFiles {
			cols := strings.SplitN(line, "\t", 2)
			if len(cols) == 2 {
				files = append(files, cols[0])
			}
		}
		err = tty_ready()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if is_tty {
		termbox.SetInputMode(termbox.InputEsc)
	}

	filter()
	draw_screen()

	// Walk and collect files recursively.
	if len(files) == 0 {
		quit = make(chan bool)
		go func() {
			filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
				if terminating {
					return errors.New("terminate")
				}
				if info == nil {
					return err
				}
				if !info.IsDir() {
					if p, err := filepath.Rel(cwd, path); err == nil {
						path = p
					}
					path = filepath.ToSlash(path)
					mutex.Lock()
					files = append(files, path)
					mutex.Unlock()
					dirty = true
					timer.Reset(duration)
				} else if strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			})
			scanning = -1
			quit <- true
		}()
	}

loop:
	for {
		update := false

		// Polling key events
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc, termbox.KeyCtrlD, termbox.KeyCtrlC:
				termbox.Close()
				os.Exit(1)
			case termbox.KeyHome, termbox.KeyCtrlA:
				cursor_x = 0
			case termbox.KeyEnd, termbox.KeyCtrlE:
				cursor_x = len(input)
			case termbox.KeyEnter:
				if cursor_y >= 0 && cursor_y < len(current) {
					if len(selected) == 0 {
						selected = append(selected, current[cursor_y].name)
					}
					break loop
				}
			case termbox.KeyArrowLeft:
				if cursor_x > 0 {
					cursor_x--
				}
			case termbox.KeyArrowRight:
				if cursor_x < len([]rune(input)) {
					cursor_x++
				}
			case termbox.KeyArrowUp, termbox.KeyCtrlK:
				if cursor_y < len(current)-1 {
					if cursor_y < height-4 {
						cursor_y++
					}
				}
			case termbox.KeyArrowDown, termbox.KeyCtrlJ:
				if cursor_y > 0 {
					cursor_y--
				}
			case termbox.KeyCtrlO:
				if cursor_y >= 0 && cursor_y < len(current) {
					*edit = true
					break loop
				}
			case termbox.KeyCtrlI:
				heading = !heading
			case termbox.KeyCtrlL:
				update = true
			case termbox.KeyCtrlU:
				cursor_x = 0
				input = []rune{}
				update = true
			case termbox.KeyCtrlW:
				part := string(input[0:cursor_x])
				rest := input[cursor_x:len(input)]
				pos := regexp.MustCompile(`\s+`).FindStringIndex(part)
				if len(pos) > 0 && pos[len(pos)-1] > 0 {
					println(pos[len(pos)-1])
					input = []rune(part[0 : pos[len(pos)-1]-1])
					input = append(input, rest...)
				} else {
					input = []rune{}
				}
				cursor_x = len(input)
				update = true
			case termbox.KeyCtrlZ:
				found := -1
				name := current[cursor_y].name
				for i, s := range selected {
					if name == s {
						found = i
						break
					}
				}
				if found == -1 {
					selected = append(selected, current[cursor_y].name)
				} else {
					selected = append(selected[:found],  selected[found+1:]...)
				}
				if !*multi {
					break loop
				}
				update = true
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if cursor_x > 0 {
					input = append(input[0:cursor_x-1], input[cursor_x:len(input)]...)
					cursor_x--
					update = true
				}
			case termbox.KeyDelete:
				if cursor_x < len([]rune(input)) {
					input = append(input[0:cursor_x], input[cursor_x+1:len(input)]...)
					update = true
				}
			default:
				if ev.Key == termbox.KeySpace {
					ev.Ch = ' '
				}
				if ev.Ch > 0 {
					out := []rune{}
					out = append(out, input[0:cursor_x]...)
					out = append(out, ev.Ch)
					input = append(out, input[cursor_x:len(input)]...)
					cursor_x++
					update = true
				}
			}
		case termbox.EventError:
			update = false
		}

		// If need to update, start timer
		if scanning != -1 {
			if update {
				dirty = true
				timer.Reset(duration)
			} else {
				timer.Reset(1)
			}
		} else {
			if update {
				filter()
			}
			draw_screen()
		}
	}
	timer.Stop()

	// Request terminating
	terminating = true
	if quit != nil {
		<-quit
	}

	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.Close()

	tty_term()

	if len(selected) == 0 {
		os.Exit(1)
	}

	if !*launcher {
		for i, f := range selected {
			selected[i] = filepath.Join(cwd, f)
		}
	}

	if *launcher {
		for _, f := range selected {
			for _, line := range launcherFiles {
				cols := strings.SplitN(line, "\t", 2)
				if len(cols) == 2 && cols[0] == f {
					stdin := os.Stdin
					stdout := os.Stdout
					var shell, shellcflag string
					if runtime.GOOS == "windows" {
						stdin, _ = os.Open("CONIN$")
						stdout, _ = os.Open("CONOUT$")
						shell = os.Getenv("COMSPEC")
						if shell == "" {
							shell = "cmd"
						}
						shellcflag = "/c"
					} else {
						stdin = os.Stdin
						shell = os.Getenv("SHELL")
						if shell == "" {
							shell = "sh"
						}
						shellcflag = "-c"
					}
					cmd := exec.Command(shell, shellcflag, strings.TrimSpace(cols[1]))
					cmd.Stdin = stdin
					cmd.Stdout = stdout
					cmd.Stderr = os.Stderr
					err = cmd.Start()
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					return
				}
			}
		}
	} else if *edit {
		err = edit_file(selected)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if *cat {
		for _, f := range selected {
			f, err := os.Open(f)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			io.Copy(os.Stdout, f)
			f.Close()
		}
	} else if *remove {
		for _, f := range selected {
			os.Remove(f)
		}
	} else if flag.NArg() > 0 {
		args := flag.Args()
		args = append(args, selected...)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Start()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cmd.Wait()
	} else {
		for _, f := range selected {
			fmt.Println(f)
		}
	}
}
