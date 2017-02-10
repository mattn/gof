# gof

Go Fuzzy

![](http://i.imgur.com/TGZJyGV.gif)

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

|Option   |Description                      |
|---------|---------------------------------|
|-c       |Cat the selected file            |
|-e       |Edit the selected file           |
|-        |Remove the selected file         |
|-l       |Launcher mode                    |
|-x       |Exit code for cancel (default: 1)|
|-d [path]|Specify root directory           |

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

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
