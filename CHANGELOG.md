# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Show the stream URL in the Now Playing panel (clickable in terminals that
  support OSC 8 hyperlinks).
- Show animated progress while updating the station list and add an
  `update_timeout` configuration option (30 seconds by default).

### Changed

- Ignore hotkeys while the station list is being updated so the UI cannot
  change mid-operation; `Ctrl+C` cancels the update.

### Deprecated

### Removed

### Fixed

### Security
