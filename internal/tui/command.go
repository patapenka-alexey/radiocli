package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// command is a user action dispatched from a keybinding.
type command int

const (
	cmdNone command = iota
	cmdMoveUp
	cmdMoveDown
	cmdPlayStation
	cmdPlaySearch
	cmdTogglePause
	cmdStop
	cmdOpenSearch
	cmdCancelSearch
	cmdBackspace
	cmdAddSearch
	cmdRefresh
	cmdRequestDelete
	cmdConfirmDelete
	cmdCancelDelete
	cmdRequestQuit
	cmdConfirmQuit
	cmdCancelQuit
	cmdSuspend
)

type keyMap map[string]command

var globalKeyMap = keyMap{
	"ctrl+c": cmdRequestQuit,
	"ctrl+z": cmdSuspend,
}

var navigateKeyMap = keyMap{
	"up":    cmdMoveUp,
	"k":     cmdMoveUp,
	"down":  cmdMoveDown,
	"j":     cmdMoveDown,
	"enter": cmdPlayStation,
	"space": cmdTogglePause,
	"s":     cmdStop,
	"/":     cmdOpenSearch,
	"u":     cmdRefresh,
	"d":     cmdRequestDelete,
	"q":     cmdRequestQuit,
}

var searchKeyMap = keyMap{
	"up":        cmdMoveUp,
	"down":      cmdMoveDown,
	"enter":     cmdPlaySearch,
	"esc":       cmdCancelSearch,
	"backspace": cmdBackspace,
	"ctrl+a":    cmdAddSearch,
	"ctrl+u":    cmdRefresh,
}

var confirmDeleteKeyMap = keyMap{
	"y":   cmdConfirmDelete,
	"n":   cmdCancelDelete,
	"esc": cmdCancelDelete,
}

var confirmQuitKeyMap = keyMap{
	"y":   cmdConfirmQuit,
	"n":   cmdCancelQuit,
	"esc": cmdCancelQuit,
}

func (m *Model) keyMap() keyMap {
	switch m.state {
	case stateSearch:
		return searchKeyMap
	case stateConfirmDelete:
		return confirmDeleteKeyMap
	case stateConfirmQuit:
		return confirmQuitKeyMap
	default:
		return navigateKeyMap
	}
}

func (m *Model) resolveCommand(token string) (command, bool) {
	if m.state == stateSearch {
		cmd, ok := searchKeyMap[token]

		return cmd, ok
	}

	return lookupKey(m.keyMap(), token)
}

// lookupKey resolves a key token. Single-character tokens fall back to their
// lowercase form so that capitals keep working for navigation and
// confirmations, while search input stays case-sensitive.
func lookupKey(bindings keyMap, token string) (command, bool) {
	if cmd, ok := bindings[token]; ok {
		return cmd, true
	}

	if utf8.RuneCountInString(token) == 1 {
		cmd, ok := bindings[strings.ToLower(token)]

		return cmd, ok
	}

	return cmdNone, false
}

func (m *Model) execute(cmd command) tea.Cmd {
	switch cmd {
	case cmdMoveUp:
		m.moveStation(-1)
	case cmdMoveDown:
		m.moveStation(1)
	case cmdPlayStation:
		m.playSelectedStation()
	case cmdPlaySearch:
		m.playSelectedSearch()
		m.state = stateNavigate
	case cmdTogglePause:
		if m.app.Status().Idle {
			m.playSelectedStation()
		} else {
			m.app.TogglePause()
		}
	case cmdStop:
		m.app.Stop()
	case cmdOpenSearch:
		m.state = stateSearch
		m.searchQuery = ""
		m.updateSearch()
	case cmdCancelSearch:
		m.state = stateNavigate
	case cmdBackspace:
		m.deleteSearchChar()
	case cmdAddSearch:
		m.addSelectedSearch()
	case cmdRefresh:
		return m.startRefresh()
	case cmdRequestDelete:
		m.requestDelete()
	case cmdConfirmDelete:
		m.confirmDelete()
	case cmdCancelDelete:
		m.cancelDelete()
	case cmdRequestQuit:
		m.requestQuit()
	case cmdConfirmQuit:
		return tea.Quit
	case cmdCancelQuit:
		m.cancelQuit()
	case cmdSuspend:
		return tea.Suspend
	}

	return nil
}

func (m *Model) deleteSearchChar() {
	if m.searchQuery == "" {
		return
	}

	r := []rune(m.searchQuery)
	m.searchQuery = string(r[:len(r)-1])
	m.updateSearch()
}
