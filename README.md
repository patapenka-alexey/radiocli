# Radio++ Terminal (Go)

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

## Configuration

The station list and mpv path are stored in `config.json`, in the same
folder as the `radiocli` binary. Set `RADIOCLI_CONFIG` to use a different path
(handy with `go run`, where the binary lives in a temporary directory).

The file is created with a small set of default stations on first run. Example:

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

The cache lives at `~/.cache/radiocli/allStations.json` (following
`$XDG_CACHE_HOME`). Set `RADIOCLI_DB` to use a different path. If the list has
never been downloaded and there is no network, the client still runs but search
returns no results.

## Files

| File | Purpose |
| --- | --- |
| `main.go` | Entry point, startup and shutdown |
| `model.go` | Bubble Tea model (UI, input handling, layout) |
| `config.go` | Configuration load/save and station management |
| `stations.go` | radio-browser.info station database (fetch, cache, search) |
| `player.go` | mpv JSON IPC controller |

## Not implemented (relative to the applet)

MPRIS integration / pausing other media players, YouTube downloads, icon/colour
appearance settings and the graphical seek bar.
