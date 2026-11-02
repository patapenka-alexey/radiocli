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

- Go 1.26 or newer (to build)
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
| `/` | Search the station database |
| `Ctrl+A` | (in search) Add the highlighted result to My Stations |
| `Enter` | (in search) Play the highlighted result |
| `Esc` | (in search) Cancel search |
| `Ctrl+U` | (in search) Update the station database |
| `d` | Delete the selected station (asks for confirmation) |
| `E` | Toggle the equalizer on/off |
| `e` | Reload equalizer presets |
| `Ctrl+E` | Select the next equalizer preset |
| `Ctrl+Z` | Suspend (playback keeps running) |
| `q` / `Ctrl+C` | Quit (asks for confirmation) |

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

The station list, mpv path and equalizer state are stored in `config.json`, at
`~/.config/radiocli/config.json` (following `$XDG_CONFIG_HOME`). Set
`RADIOCLI_CONFIG` to use a different path.

The file is written when you add (`Ctrl+A` while searching) or delete (`d`) a
station, or when the equalizer is toggled, reloaded or changed; until then the
in-memory defaults below are used. Example:

```json
{
  "stations": [
    { "name": "Smooth Chill", "url": "https://media-ssl.musicradio.com/ChillMP3" }
  ],
  "mpv_path": "mpv",
  "equalizer_enabled": false,
  "equalizer_preset": "wa_Rock",
  "equalizer_mixin": 0.25
}
```

To point the client at a different mpv binary, change `mpv_path`. The
equalizer state is saved automatically whenever it is toggled, reloaded or
changed. `equalizer_mixin` controls how much original signal is mixed back in
after equalizing (0.0 applies only the equalized signal, 1.0 disables
equalization); the default is `0.25`.

Playback volume is controlled by the system mixer; use your desktop's volume
keys or mixer applet.

## Equalizer

The equalizer uses MOC's preset format. On startup radiocli reads presets from
`~/.config/radiocli/eqsets`; if that directory is missing or empty, it falls
back to the presets bundled into the binary. Set `RADIOCLI_EQSETS` to use
another directory.

`E` toggles the filter, `Ctrl+E` cycles presets and `e` re-reads the preset
directory (useful after editing a preset). Presets are applied through mpv's
`af` property using FFmpeg's octave-width peaking equalizer, with MOC's
preamp line mapped to a `volume` filter. MOC's Mixin setting is supported via
the visible `equalizer_mixin` config option: the signal is split, equalized on
one branch, and mixed back with the original using that ratio.

The bundled presets originate from the MOC eqsets collection described in
`internal/equalizer/eqsets/README.md`.

## Station search

Search uses a station list fetched directly from the
[radio-browser.info](https://www.radio-browser.info) API - the same source the
Cinnamon applet queries when you press "Update Radio Stationlist". The list is
downloaded on first run (and whenever you press `Ctrl+U` while searching),
then cached so search works offline. Mirror hosts are tried with
exponential backoff if the first one is unavailable.

The cache lives at `~/.config/radiocli/allStations.json` (following
`$XDG_CONFIG_HOME`), next to `config.json`. Set `RADIOCLI_DB` to use a
different path. If the list has never been downloaded and there is no network,
the client still runs but search returns no results.

## Architecture

The code is organized as ports and adapters:

- `internal/ports` — contracts used by the application core
  (`StationStore`, `Catalog`, `Player`).
- `internal/app` — application service/facade for playback, station management
  and search; frontends call it instead of touching adapters directly.
- `internal/config`, `internal/catalog`, `internal/player` — infrastructure
  adapters for the filesystem, radio-browser.info and mpv.
- `internal/equalizer` — MOC preset parser, preset library and mpv filter
  builder.
- `internal/tui` — Bubble Tea presentation layer; keeps only UI state
  (selection, query, modal state, scroll offsets).
- `internal/bootstrap` — composition root that wires everything together.

Playback is event-driven: `player.MPV` publishes observed property changes on a
channel, the TUI subscribes with a `tea.Cmd`, and the channel closing signals
that mpv exited. The UI is an explicit state machine (`navigate`, `search`,
`confirm delete`, `confirm quit`) with a per-state command keymap. Catalog
refreshes use cache-aside with mirror failover, exponential backoff and jitter.

## Files

| Path | Purpose |
| --- | --- |
| `main.go` | Entry point and argument dispatch |
| `internal/bootstrap` | Composition root (`Build`, `Close`) |
| `internal/app` | Application service |
| `internal/ports` | Interfaces between core and adapters |
| `internal/station` | Station domain type |
| `internal/tui` | Bubble Tea model, state machine, commands, layout |
| `internal/config` | Configuration load/save and station management |
| `internal/catalog` | radio-browser.info fetch, cache and search |
| `internal/player` | mpv JSON IPC controller and event stream |
| `internal/equalizer` | MOC preset parsing and mpv equalizer filters |
| `internal/control` | Remote-control subcommands (`pause` / `unpause`) |
| `internal/paths` | Config, cache and socket locations |
| `Makefile` | Build and install targets |

