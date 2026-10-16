# radiocli

A terminal client that re-implements the core of the Cinnamon **Radio++**
applet (`radio@driglu4it`), written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea). It plays
internet-radio streams through [`mpv`](https://mpv.io), lets you manage your
own station list and searches the online station database from
[radio-browser.info](https://www.radio-browser.info).

Unlike the applet, it does **not** require Cinnamon, the `mpv-mpris` plugin or
DBus: playback is controlled directly through mpv's JSON IPC socket.

## Requirements

- Go 1.24 or newer (to build)
- `mpv` available on `PATH`

## Building and running

```sh
go build -o radiocli .
./radiocli
```

or run directly without producing a binary:

```sh
go run .
```

A `Makefile` wraps these steps:

```sh
make                  # build ./radiocli
make && sudo make install   # then install to /usr/local/bin
make uninstall        # remove the installed binary
make clean            # remove the local binary
```

`install` never builds, so `sudo` does not need `go` on root's `PATH`; always
run `make` first. Install path can be changed with
`make install PREFIX=$HOME/.local`.

## Keybindings

| Key | Action |
| --- | --- |
| `↑` / `↓` or `k` / `j` | Move the selection |
| `Enter` | Play the selected station |
| `Space` | Play/pause |
| `s` | Stop playback |
| `←` / `→` | Seek back / forward 10 seconds |
| `/` | Search the station database |
| `a` | (in search) Add the highlighted result to My Stations |
| `Enter` | (in search) Play the highlighted result |
| `Esc` | (in search) Cancel search |
| `u` | Update the station database from radio-browser.info |
| `d` | Delete the selected station (asks for confirmation) |
| `q` / `Ctrl+C` | Quit |

## Remote control

The running instance opens an mpv JSON IPC socket, so you can pause and resume
playback from another terminal without touching the UI:

```sh
radiocli pause      # pause the running instance
radiocli unpause    # resume it
```

The commands are silent on success and exit non-zero with an error if no
`radiocli` instance is running. The UI updates immediately, since it observes
mpv's `pause` property.

## Configuration

The station list and mpv path are stored in `config.json`, at
`~/.config/radiocli/config.json` (following `$XDG_CONFIG_HOME`). Set
`RADIOCLI_CONFIG` to use a different path.

The file is only written when you add (`a`) or delete (`d`) a station; until
then the in-memory defaults below are used. Example:

```json
{
  "stations": [
    { "name": "Smooth Chill", "url": "https://media-ssl.musicradio.com/ChillMP3" }
  ],
  "mpv_path": "mpv"
}
```

To point the client at a different mpv binary, change `mpv_path`.

Playback volume is controlled by the system mixer; use your desktop's volume
keys or mixer applet.

## Station search

Search uses a station list fetched directly from the
[radio-browser.info](https://www.radio-browser.info) API - the same source the
Cinnamon applet queries when you press "Update Radio Stationlist". The list is
downloaded on first run (and whenever you press `u`), then cached so search
works offline.

The cache lives at `~/.config/radiocli/allStations.json` (following
`$XDG_CONFIG_HOME`), next to `config.json`. Set `RADIOCLI_DB` to use a
different path. If the list has never been downloaded and there is no network,
the client still runs but search returns no results.

## Files

| File | Purpose |
| --- | --- |
| `main.go` | Entry point, startup and shutdown |
| `model.go` | Bubble Tea model (UI, input handling, layout) |
| `config.go` | Configuration load/save and station management |
| `stations.go` | radio-browser.info station database (fetch, cache, search) |
| `player.go` | mpv JSON IPC controller |
| `control.go` | Remote-control subcommands (`pause` / `unpause`) |
| `Makefile` | Build and install targets |

## Not implemented (relative to the applet)

MPRIS integration / pausing other media players, YouTube downloads, icon/colour
appearance settings and the graphical seek bar.
