package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/transform"

	"github.com/mattn/go-colorable"
	enc "github.com/mattn/go-encoding"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/saracen/walker"
)

const name = "gof"

const version = "0.0.10"

var revision = "HEAD"

type matched struct {
	name     string
	pos1     int
	pos2     int
	selected bool
	index    int
}

var (
	input            = []rune{}
	files            []string
	selected         = []string{}
	heading          = false
	current          []matched
	cursorX, cursorY int
	offset           int
	width, height    int
	mutex            sync.Mutex
	dirty            = false
	duration         = 20 * time.Millisecond
	timer            *time.Timer
	scanning         = 0
	ignorere         *regexp.Regexp
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func tprint(x, y int, fg, bg termbox.Attribute, msg string) {
	for _, c := range []rune(msg) {
		termbox.SetCell(x, y, c, fg, bg)
		x += runewidth.RuneWidth(c)
	}
}

func tprintf(x, y int, fg, bg termbox.Attribute, format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	tprint(x, y, fg, bg, s)
}

func filter(fuzzy bool) {
	mutex.Lock()
	fs := files
	inp := input
	sel := selected
	mutex.Unlock()

	var tmp []matched
	if len(inp) == 0 {
		tmp = make([]matched, len(fs))
		for n, f := range fs {
			prevSelected := false
			for _, s := range sel {
				if f == s {
					prevSelected = true
					break
				}
			}
			tmp[n] = matched{
				name:     f,
				pos1:     -1,
				pos2:     -1,
				selected: prevSelected,
				index:    n,
			}
		}
	} else if fuzzy {
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
			prevSelected := false
			for _, s := range sel {
				if f == s {
					prevSelected = true
					break
				}
			}
			tmp = append(tmp, matched{
				name:     f,
				pos1:     len([]rune(f[0:ms[0][2]])),
				pos2:     len([]rune(f[0:ms[0][3]])),
				selected: prevSelected,
				index:    len(tmp),
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
			prevSelected := false
			for _, s := range sel {
				if f == s {
					prevSelected = true
					break
				}
			}
			pos1 := len([]rune(f[:pos]))
			tmp = append(tmp, matched{
				name:     f,
				pos1:     pos1,
				pos2:     pos1 + len(inp),
				selected: prevSelected,
				index:    len(tmp),
			})
		}
	}
	if len(inp) > 0 {
		sort.Slice(tmp, func(i, j int) bool {
			li, lj := tmp[i].pos2-tmp[i].pos1, tmp[j].pos2-tmp[j].pos1
			return li < lj || li == lj && tmp[i].index < tmp[j].index
		})
	}

	mutex.Lock()
	defer mutex.Unlock()
	current = tmp
	selected = sel
	if cursorY < 0 {
		cursorY = 0
	}
	if cursorY >= len(current) {
		cursorY = len(current) - 1
	}
	if cursorY >= 0 && height > 0 {
		if cursorY < offset {
			offset = cursorY
		} else if offset < cursorY-(height-3) {
			offset = cursorY - (height - 3)
		}
		if len(current)-(height-3)-1 < offset {
			offset = len(current) - (height - 3) - 1
			if offset < 0 {
				offset = 0
			}
		}
	} else {
		offset = 0
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
	for n := offset; n <= height-3+offset; n++ {
		if n >= len(current) {
			break
		}
		x, y, w := 2, height-3-(n-offset), 0
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
					termbox.SetCell(x, y, c, termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault)
				} else if cursorY == n {
					termbox.SetCell(x, y, c, termbox.ColorYellow|termbox.AttrBold|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, y, c, termbox.ColorGreen|termbox.AttrBold, termbox.ColorDefault)
				}
			} else {
				if selected {
					termbox.SetCell(x, y, c, termbox.ColorRed, termbox.ColorDefault)
				} else if cursorY == n {
					termbox.SetCell(x, y, c, termbox.ColorYellow|termbox.AttrUnderline, termbox.ColorDefault)
				} else {
					termbox.SetCell(x, y, c, termbox.ColorDefault, termbox.ColorDefault)
				}
			}
			x += w
		}
	}
	if cursorY >= 0 {
		tprint(0, height-3-(cursorY-offset), termbox.ColorRed|termbox.AttrBold, termbox.ColorDefault, "> ")
	}
	if scanning >= 0 {
		tprint(0, height-2, termbox.ColorGreen, termbox.ColorDefault, string([]rune("-\\|/")[scanning%4]))
		scanning++
	}
	tprintf(2, height-2, termbox.ColorDefault, termbox.ColorDefault, "%d/%d(%d)", len(current), len(files), len(selected))
	tprint(0, height-1, termbox.ColorBlue|termbox.AttrBold, termbox.ColorDefault, "> ")
	tprint(2, height-1, termbox.ColorDefault|termbox.AttrBold, termbox.ColorDefault, string(input))
	termbox.SetCursor(2+runewidth.StringWidth(string(input[0:cursorX])), height-1)
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

func readLines(ctx context.Context, wg *sync.WaitGroup, r io.Reader) {
	defer wg.Done()

	var buf *bufio.Reader
	if se := os.Getenv("GOF_STDIN_ENC"); se != "" {
		if e := enc.GetEncoding(se); e != nil {
			buf = bufio.NewReader(transform.NewReader(r, e.NewDecoder().Transformer))
		} else {
			buf = bufio.NewReader(r)
		}
	} else {
		buf = bufio.NewReader(r)
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
	scanning = -1
	mutex.Unlock()
}

func listFiles(ctx context.Context, wg *sync.WaitGroup, cwd string) {
	defer wg.Done()

	n := 0
	cb := walker.WithErrorCallback(func(pathname string, err error) error {
		return nil
	})
	fn := func(path string, info os.FileInfo) error {
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
	walker.WalkWithContext(ctx, cwd, fn, cb)
	mutex.Lock()
	dirty = true
	timer.Reset(duration)
	scanning = -1
	mutex.Unlock()
}

func main() {
	var open2edit bool

	var fuzzy bool
	var root string
	var exit int
	var action string
	var tapi bool
	var tapiFunc string
	var ignore string
	var showVersion bool

	flag.BoolVar(&fuzzy, "f", false, "Fuzzy match")
	flag.StringVar(&root, "d", "", "Root directory")
	flag.IntVar(&exit, "x", 1, "Exit code for cancel")
	flag.StringVar(&action, "a", "", "Action keys")
	flag.BoolVar(&tapi, "t", false, "Open via Vim's Terminal API")
	flag.StringVar(&tapiFunc, "tf", "", "Terminal API's function name")
	flag.StringVar(&ignore, "i", env(`GOF_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore pattern")
	flag.BoolVar(&showVersion, "v", false, "Print the version")
	flag.Parse()

	if showVersion {
		fmt.Printf("%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return
	}

	defer colorable.EnableColorsStdout(nil)()

	var err error

	// Make regular expression pattern to ignore files.
	if ignore != "" {
		ignorere, err = regexp.Compile(ignore)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// Setup terminal API.
	tapi = tapi || tapiFunc != ""
	if tapi {
		if os.Getenv("VIM_TERMINAL") == "" {
			fmt.Fprintln(os.Stderr, "-t,-tf option is only available inside Vim's terminal window")
			os.Exit(1)
		}
	}

	// Make sure current directory.
	cwd := ""
	if root == "" {
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		if runtime.GOOS == "windows" && strings.HasPrefix(root, "/") {
			cwd, _ = os.Getwd()
			cwd = filepath.Join(filepath.VolumeName(cwd), root)
		} else {
			cwd, err = filepath.Abs(root)
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

	redrawFunc := func() {
		mutex.Lock()
		d := dirty
		mutex.Unlock()
		if d {
			filter(fuzzy)
			drawLines()

			mutex.Lock()
			dirty = false
			mutex.Unlock()
		} else {
			drawLines()
		}
	}
	timer = time.AfterFunc(0, redrawFunc)
	timer.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	isTty := isTerminal()
	ctx, cancel := context.WithCancel(context.Background())
	if isTty {
		// Walk and collect files recursively.
		go listFiles(ctx, &wg, cwd)
	} else {
		// Read lines from stdin.
		go readLines(ctx, &wg, os.Stdin)
	}

	if !isTty {
		err = startTerminal()
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
	termbox.SetInputMode(termbox.InputEsc)

	redrawFunc()
	actionKey := ""

loop:
	for {
		update := false

		// Polling key events
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			for _, ka := range strings.Split(action, ",") {
				for i, kv := range actionKeys {
					if ev.Key != kv {
						continue
					}
					ak := fmt.Sprintf("ctrl-%c", 'a'+i)
					if ka != ak {
						continue
					}
					if cursorY >= 0 && cursorY < len(current) {
						if len(selected) == 0 {
							selected = append(selected, current[cursorY].name)
						}
						actionKey = ak
						break loop
					}
				}
			}
			switch ev.Key {
			case termbox.KeyEsc, termbox.KeyCtrlD, termbox.KeyCtrlC:
				termbox.Close()
				os.Exit(exit)
			case termbox.KeyHome, termbox.KeyCtrlA:
				cursorX = 0
			case termbox.KeyEnd, termbox.KeyCtrlE:
				cursorX = len(input)
			case termbox.KeyEnter:
				if cursorY >= 0 && cursorY < len(current) {
					if len(selected) == 0 {
						selected = append(selected, current[cursorY].name)
					}
					break loop
				}
			case termbox.KeyCtrlO:
				if cursorY >= 0 && cursorY < len(current) {
					if len(selected) == 0 {
						selected = append(selected, current[cursorY].name)
					}
					open2edit = true
					break loop
				}
			case termbox.KeyArrowLeft:
				if cursorX > 0 {
					cursorX--
				}
			case termbox.KeyArrowRight:
				if cursorX < len([]rune(input)) {
					cursorX++
				}
			case termbox.KeyArrowUp, termbox.KeyCtrlK, termbox.KeyCtrlP:
				if cursorY < len(current)-1 {
					cursorY++
					if offset < cursorY-(height-3) {
						offset = cursorY - (height - 3)
					}
				}
			case termbox.KeyArrowDown, termbox.KeyCtrlJ, termbox.KeyCtrlN:
				if cursorY > 0 {
					cursorY--
					if cursorY < offset {
						offset = cursorY
					}
				}
			case termbox.KeyCtrlI:
				heading = !heading
			case termbox.KeyCtrlL:
				update = true
			case termbox.KeyCtrlU:
				cursorX = 0
				input = []rune{}
				update = true
			case termbox.KeyCtrlW:
				part := string(input[0:cursorX])
				rest := input[cursorX:len(input)]
				pos := regexp.MustCompile(`\s+`).FindStringIndex(part)
				if len(pos) > 0 && pos[len(pos)-1] > 0 {
					input = []rune(part[0 : pos[len(pos)-1]-1])
					input = append(input, rest...)
				} else {
					input = []rune{}
				}
				cursorX = len(input)
				update = true
			case termbox.KeyCtrlZ:
				found := -1
				name := current[cursorY].name
				for i, s := range selected {
					if name == s {
						found = i
						break
					}
				}
				if found == -1 {
					selected = append(selected, current[cursorY].name)
				} else {
					selected = append(selected[:found], selected[found+1:]...)
				}
				update = true
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if cursorX > 0 {
					input = append(input[0:cursorX-1], input[cursorX:len(input)]...)
					cursorX--
					update = true
				}
			case termbox.KeyDelete:
				if cursorX < len([]rune(input)) {
					input = append(input[0:cursorX], input[cursorX+1:len(input)]...)
					update = true
				}
			case termbox.KeyCtrlR:
				fuzzy = !fuzzy
				update = true
			default:
				if ev.Key == termbox.KeySpace {
					ev.Ch = ' '
				}
				if ev.Ch > 0 {
					out := []rune{}
					out = append(out, input[0:cursorX]...)
					out = append(out, ev.Ch)
					input = append(out, input[cursorX:len(input)]...)
					cursorX++
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
				filter(fuzzy)
			}
			drawLines()
		}
	}
	timer.Stop()

	// Request terminating
	cancel()

	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	termbox.Close()
	stopTerminal()

	wg.Wait()

	if len(selected) == 0 {
		os.Exit(exit)
	}

	if tapi {
		for _, f := range selected {
			command := make([]interface{}, 0, 3)
			if tapiFunc != "" {
				command = append(command, "call", tapiFunc, newVimTapiCall(cwd, f, actionKey))
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
		if action != "" {
			fmt.Println(actionKey)
		}
		fArg := []string{}
		if root != "" {
			for _, f := range selected {
				fmt.Println(filepath.Join(root, f))
				fArg = append(fArg, filepath.Join(root, f))
			}
		} else {
			for _, f := range selected {
				fmt.Println(f)
				fArg = append(fArg, f)
			}

		}
		if open2edit {
			cmd := exec.Command("vim", fArg...)
			fi, err := os.Stdin.Stat()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fo, err := os.Stdout.Stat()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if strings.Index(fi.Mode().String(), "p") != 0 && strings.Index(fo.Mode().String(), "p") != 0 {
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if err != nil {
					fmt.Println(err)
				}
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
