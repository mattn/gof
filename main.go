package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/transform"

	enc "github.com/mattn/go-encoding"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/saracen/walker"
)

const name = "gof"

const version = "0.0.4"

var revision = "HEAD"

var (
	fuzzy               = flag.Bool("f", false, "Fuzzy match")
	root                = flag.String("d", "", "Root directory")
	exit                = flag.Int("x", 1, "Exit code for cancel")
	action              = flag.String("a", "", "Action keys")
	terminalApi         = flag.Bool("t", false, "Open via Vim's Terminal API")
	terminalApiFuncname = flag.String("tf", "", "Terminal API's function name")
	ignore              = flag.String("i", env(`GOF_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore pattern")
	showVersion         = flag.Bool("v", false, "Print the version")
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

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

type matched struct {
	name     string
	pos1     int
	pos2     int
	selected bool
}

var (
	input              = []rune{}
	files              []string
	selected           = []string{}
	heading            = false
	current            []matched
	cursor_x, cursor_y int
	width, height      int
	mutex              sync.Mutex
	launcherFiles      = []string{}
	dirty              = false
	duration           = 20 * time.Millisecond
	timer              *time.Timer
	scanning           = 0
	drawing            = false
	terminating        = false
	ignorere           *regexp.Regexp
)

func filter() {
	mutex.Lock()
	fs := files
	inp := input
	sel := selected
	mutex.Unlock()

	var tmp []matched
	if len(inp) == 0 {
		tmp = make([]matched, len(fs))
		for n, f := range fs {
			prev_selected := false
			for _, s := range sel {
				if f == s {
					prev_selected = true
					break
				}
			}
			tmp[n] = matched{
				name:     f,
				pos1:     -1,
				pos2:     -1,
				selected: prev_selected,
			}
		}
	} else if *fuzzy {
		pat := "(?i)(?:.*)("
		for _, r := range []rune(inp) {
			pat += regexp.QuoteMeta(string(r)) + ".*?"
		}
		pat += ")"
		re := regexp.MustCompile(pat)

		tmp = make([]matched, 0, len(fs))
		for _, f := range fs {
			ms := re.FindAllStringSubmatchIndex(f, 1)
			if len(ms) != 1 || len(ms[0]) != 4 {
				continue
			}
			prev_selected := false
			for _, s := range sel {
				if f == s {
					prev_selected = true
					break
				}
			}
			tmp = append(tmp, matched{
				name:     f,
				pos1:     len([]rune(f[0:ms[0][2]])),
				pos2:     len([]rune(f[0:ms[0][3]])),
				selected: prev_selected,
			})
		}
	} else {
		tmp = make([]matched, 0, len(fs))
		inpl := strings.ToLower(string(inp))
		for _, f := range fs {
			var pos int
			if lf := strings.ToLower(f); len(f) == len(lf) {
				pos = strings.Index(lf, inpl)
			} else {
				pos = bytes.Index([]byte(f), []byte(string(inp)))
			}
			if pos == -1 {
				continue
			}
			prev_selected := false
			for _, s := range sel {
				if f == s {
					prev_selected = true
					break
				}
			}
			pos1 := len([]rune(f[:pos]))
			tmp = append(tmp, matched{
				name:     f,
				pos1:     pos1,
				pos2:     pos1 + len(inp),
				selected: prev_selected,
			})
		}
	}
	if len(inp) > 0 {
		sort.Slice(tmp, func(i, j int) bool {
			li, lj := tmp[i].pos2-tmp[i].pos1, tmp[j].pos2-tmp[j].pos1
			return li < lj || li == lj && tmp[i].pos1 < tmp[j].pos1
		})
	}

	mutex.Lock()
	defer mutex.Unlock()
	current = tmp
	selected = sel
	if cursor_y < 0 {
		cursor_y = 0
	}
	if cursor_y >= len(current) {
		cursor_y = len(current) - 1
	}
}

func drawLines() {
	defer func() {
		recover()
	}()
	mutex.Lock()
	defer mutex.Unlock()

	width, height = termbox.Size()
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	pat := ""
	for _, r := range input {
		pat += regexp.QuoteMeta(string(r)) + ".*?"
	}
	for n := 0; n < height-1; n++ {
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
					termbox.SetCell(x, height-3-n, c, termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault)
				} else if cursor_y == n {
					termbox.SetCell(x, height-3-n, c, termbox.ColorYellow|termbox.AttrBold|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, height-3-n, c, termbox.ColorGreen|termbox.AttrBold, termbox.ColorDefault)
				}
			} else {
				if selected {
					termbox.SetCell(x, height-3-n, c, termbox.ColorRed, termbox.ColorDefault)
				} else if cursor_y == n {
					termbox.SetCell(x, height-3-n, c, termbox.ColorYellow|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, height-3-n, c, termbox.ColorDefault, termbox.ColorDefault)
				}
			}
			x += w
		}
	}
	if cursor_y >= 0 {
		print_tb(0, height-3-cursor_y, termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault, "> ")
	}
	if scanning >= 0 {
		print_tb(0, height-2, termbox.ColorGreen, termbox.ColorDefault, string([]rune("-\\|/")[scanning%4]))
		scanning++
	}
	printf_tb(2, height-2, termbox.ColorDefault, termbox.ColorDefault, "%d/%d(%d)", len(current), len(files), len(selected))
	print_tb(0, height-1, termbox.ColorBlue|termbox.AttrBold, termbox.ColorDefault, "> ")
	print_tb(2, height-1, termbox.ColorDefault|termbox.AttrBold, termbox.ColorDefault, string(input))
	termbox.SetCursor(2+runewidth.StringWidth(string(input[0:cursor_x])), height-1)
	termbox.Flush()
}

var actionKeys = []termbox.Key{
	termbox.KeyCtrlA,
	termbox.KeyCtrlB,
	termbox.KeyCtrlC,
	termbox.KeyCtrlD,
	termbox.KeyCtrlE,
	termbox.KeyCtrlF,
	termbox.KeyCtrlG,
	termbox.KeyCtrlH,
	termbox.KeyCtrlI,
	termbox.KeyCtrlJ,
	termbox.KeyCtrlK,
	termbox.KeyCtrlL,
	termbox.KeyCtrlM,
	termbox.KeyCtrlN,
	termbox.KeyCtrlO,
	termbox.KeyCtrlP,
	termbox.KeyCtrlQ,
	termbox.KeyCtrlR,
	termbox.KeyCtrlS,
	termbox.KeyCtrlT,
	termbox.KeyCtrlU,
	termbox.KeyCtrlV,
	termbox.KeyCtrlW,
	termbox.KeyCtrlX,
	termbox.KeyCtrlY,
	termbox.KeyCtrlZ,
}

func readLines(quit chan bool) {
	defer close(quit)

	var buf *bufio.Reader
	if se := os.Getenv("GOF_STDIN_ENC"); se != "" {
		if e := enc.GetEncoding(se); e != nil {
			buf = bufio.NewReader(transform.NewReader(os.Stdin, e.NewDecoder().Transformer))
		} else {
			buf = bufio.NewReader(os.Stdin)
		}
	} else {
		buf = bufio.NewReader(os.Stdin)
	}
	files = []string{}

	n := 0
	for {
		b, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		mutex.Lock()
		files = append(files, string(b))
		n++
		if n%1000 == 0 {
			dirty = true
			timer.Reset(duration)
		}
		mutex.Unlock()
	}
	mutex.Lock()
	dirty = true
	timer.Reset(duration)
	mutex.Unlock()
	scanning = -1
	quit <- true
}

func listFiles(cwd string, quit chan bool) {
	defer close(quit)

	n := 0
	cb := walker.WithErrorCallback(func(pathname string, err error) error {
		return nil
	})
	fn := func(path string, info os.FileInfo) error {
		if terminating {
			return errors.New("terminate")
		}
		path = filepath.Clean(path)
		if p, err := filepath.Rel(cwd, path); err == nil {
			path = p
		}
		if path == "." {
			return nil
		}
		base := filepath.Base(path)
		if ignorere != nil && ignorere.MatchString(base) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		path = filepath.ToSlash(path)
		mutex.Lock()
		files = append(files, path)
		n++
		if n%1000 == 0 {
			dirty = true
			timer.Reset(duration)
		}
		mutex.Unlock()
		return nil
	}
	walker.Walk(cwd, fn, cb)
	mutex.Lock()
	dirty = true
	timer.Reset(duration)
	mutex.Unlock()
	scanning = -1
	quit <- true
}

func redrawFunc() {
	mutex.Lock()
	d := dirty
	mutex.Unlock()
	if d {
		filter()
		drawLines()
		mutex.Lock()
		dirty = false
		mutex.Unlock()
	} else {
		drawLines()
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return
	}

	var err error
	cwd := ""

	*terminalApi = *terminalApi || *terminalApiFuncname != ""
	if *terminalApi {
		if os.Getenv("VIM_TERMINAL") == "" {
			fmt.Fprintln(os.Stderr, "-t,-tf option is only available inside Vim's terminal window")
			os.Exit(1)
		}
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

	if *ignore != "" {
		ignorere, err = regexp.Compile(*ignore)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var quit chan bool

	timer = time.AfterFunc(0, redrawFunc)
	timer.Stop()

	quit = make(chan bool)
	isTty := isTerminal()
	if !isTty {
		// Read lines from stdin.
		go readLines(quit)

		err = startTerminal()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		// Walk and collect files recursively.
		go listFiles(cwd, quit)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if isTty {
		termbox.SetInputMode(termbox.InputEsc)
	}

	redrawFunc()
	actionKey := ""

loop:
	for {
		update := false

		// Polling key events
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			for _, ka := range strings.Split(*action, ",") {
				for i, kv := range actionKeys {
					if ev.Key == kv {
						ak := fmt.Sprintf("ctrl-%c", 'a'+i)
						if ka == ak {
							if cursor_y >= 0 && cursor_y < len(current) {
								if len(selected) == 0 {
									selected = append(selected, current[cursor_y].name)
								}
								actionKey = ak
								break loop
							}
						}
					}
				}
			}
			switch ev.Key {
			case termbox.KeyEsc, termbox.KeyCtrlD, termbox.KeyCtrlC:
				termbox.Close()
				os.Exit(*exit)
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
			case termbox.KeyArrowUp, termbox.KeyCtrlK, termbox.KeyCtrlP:
				if cursor_y < len(current)-1 {
					if cursor_y < height-3 {
						cursor_y++
					}
				}
			case termbox.KeyArrowDown, termbox.KeyCtrlJ, termbox.KeyCtrlN:
				if cursor_y > 0 {
					cursor_y--
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
					selected = append(selected[:found], selected[found+1:]...)
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
			case termbox.KeyCtrlR:
				*fuzzy = !*fuzzy
				update = true
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
				mutex.Lock()
				dirty = true
				timer.Reset(duration)
				mutex.Unlock()
			} else {
				timer.Reset(1)
			}
		} else {
			if update {
				filter()
			}
			drawLines()
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
	stopTerminal()

	if len(selected) == 0 {
		os.Exit(*exit)
	}

	if *terminalApi {
		for _, f := range selected {
			command := make([]interface{}, 0, 3)
			if *terminalApiFuncname != "" {
				command = append(command, "call", *terminalApiFuncname, newVimTapiCall(cwd, f, actionKey))
			} else {
				if !filepath.IsAbs(f) {
					f = filepath.Join(cwd, f)
				}
				command = append(command, "drop", f)
			}
			b, err := json.Marshal(command)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("\x1b]51;%s\x07", string(b))
		}
	} else {
		if *action != "" {
			fmt.Println(actionKey)
		}
		if *root != "" {
			for _, f := range selected {
				fmt.Println(filepath.Join(*root, f))
			}
		} else {
			for _, f := range selected {
				fmt.Println(f)
			}

		}
	}
}

type vimTapiCall struct {
	RootDir   string `json:"root_dir"`
	Filename  string `json:"filename"`
	Fullpath  string `json:"fullpath"`
	ActionKey string `json:"action_key"`
}

func newVimTapiCall(rootDir, filename, actionKey string) *vimTapiCall {
	fullpath := filename
	if !filepath.IsAbs(filename) {
		fullpath = filepath.Join(rootDir, filename)
	}
	return &vimTapiCall{RootDir: rootDir, Filename: filename, Fullpath: fullpath, ActionKey: actionKey}
}
