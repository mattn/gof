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

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
