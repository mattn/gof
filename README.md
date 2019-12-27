# gof

Go Fuzzy

![](http://i.imgur.com/TGZJyGV.gif)

[Open files in Vim directly (inside Vim terminal)](#vim-terminal-api)

![](https://i.imgur.com/g81MCyr.gif)

## Installation

    $ go get github.com/mattn/gof

## Feature

* Faster and startup
* Working on windows

## Usage

* Glob files and edit the selected file with vim.

```sh
$ vim `gof`
```

* Run gof and type `CTRL-O`, then start to edit with editor.

```sh
$ gof
```

* Read from stdin

```sh
$ find /tmp | gof
```

## Key Assign

|Key                                               |Description                         |
|--------------------------------------------------|------------------------------------|
|<kbd>CTRL-K</kbd>,<kbd>ARROW-UP</kbd>             |Move-up line                        |
|<kbd>CTRL-J</kbd>,<kbd>ARROW-DOWN</kbd>           |Move-down line                      |
|<kbd>CTRL-A</kbd>,<kbd>HOME</kbd>                 |Go to head of prompt                |
|<kbd>CTRL-E</kbd>,<kbd>END</kbd>                  |Go to trail of prompt               |
|<kbd>ARROW-LEFT</kbd>                             |Move-left cursor                    |
|<kbd>ARROW-RIGHT</kbd>                            |Move-right cursor                   |
|<kbd>CTRL-O</kbd>                                 |Edit file selected                  |
|<kbd>CTRL-I</kbd>                                 |Toggle view header/trailing of lines|
|<kbd>CTRL-L</kbd>                                 |Redraw                              |
|<kbd>CTRL-U</kbd>                                 |Clear prompt                        |
|<kbd>CTRL-W</kbd>                                 |Remove backward word                |
|<kbd>BS</kbd>                                     |Remove backward character           |
|<kbd>DEL</kbd>                                    |Delete character on the cursor      |
|<kbd>CTRL-Z</kbd>                                 |Toggle selection                    |
|<kbd>Enter</kbd>                                  |Decide                              |
|<kbd>CTRL-D</kbd>,<kbd>CTRL-C</kbd>,<kbd>ESC</kbd>|Cancel                              |

## Options

|Option     |Description                             |
|-----------|----------------------------------------|
|-c         |Cat the selected file                   |
|-e         |Edit the selected file                  |
|-          |Remove the selected file                |
|-l         |Launcher mode                           |
|-x         |Exit code for cancel (default: 1)       |
|-d [path]  |Specify root directory                  |
|-t         |Open via Vim's Terminal API             |
|-T [prefix]|Terminal API's prefix (default: "Tapi_")|

## Launcher Mode

Put `~/.gof-launcher`

```
[name]	[command]
```

`name` and `command` should be separated by TAB. `gof -l` launch `command`s for selected `name`. Below is a my `.gof-launcher` file.

```
Vim	gvim
Emacs	emacs
GIMP	gimp
```

## Vim Terminal API

* `gof -t` or `gof -T [prefix]` opens selected files in Vim using [Terminal
  API](https://vim-jp.org/vimdoc-en/terminal.html#terminal-api).  This option is
  ignored when `-l`, `-e`, `-c`, `-r`, or 1 or more non-option argument were
  supplied

* If you want to add `-t` option automatically whether you are inside Vim
  terminal or not, you can define alias like this

```sh
gof() {
  if [ "$VIM_TERMINAL" ]; then
    gof -t "$@"
  else
    gof "$@"
  fi
}
```

* If you use `term_setapi()` in your Vim, use `gof -T [prefix]` to specify the
  prefix (but maybe you never use this function :sweat_smile:)

* You can define utility Vim command `:Gof`. Quickly calls `gof -t` command and
  opens selected files in Vim buffer

```vim
if executable('gof')
  command! -nargs=* Gof term ++close gof -t
endif
```

![](https://i.imgur.com/dJ8ypKT.gif)

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
