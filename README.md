# whatscli

A command line interface for whatsapp, based on [go-whatsapp](https://github.com/Rhymen/go-whatsapp) and [tview](https://github.com/rivo/tview)

![whatscli-screenshot](/doc/screenshot.png?raw=true "WhatsCLI 0.6.5")

## Features

Things that work.

- Sending and receiving WhatsApp messages in a command line app
- Connects through the Web App API without a browser
- Uses QR code for simple setup
- Allows downloading and opening image/video/audio/document attachments
- Allows sending documents
- Allows color customization
- Allows basic group management
- Supports desktop notifications
- Binaries for Windows, Mac, Linux and RaspBerry Pi

### Caveats

Heres some things you might expect to work that don't. Plus some other things I should mention.

- Only shows existing chats
- No auto-reconnect when connection drops
- No automation of messages, no sending of messages through shell commands
- FaceBook obviously doesn't endorse or like these kinds of apps and they're likely to break when FaceBook changes stuff in their web app

## Installation / Usage

How to get it running and how to use it

### Latest Release

Always fresh, always up to date.

- Download a release
- Put the binary in your PATH (optional)
- Run with `whatscli` (or double-click)
- Scan the QR code with WhatsApp on your phone (resize shell or change font size to see whole code)

### Package Managers

Some ways to install via package managers are supported but the installed version might be out of date.

#### MacOS (homebrew)

- `brew install normen/tap/whatscli`

#### Arch Linux (AUR)

- `https://aur.archlinux.org/packages/whatscli/`

## Development

This app started as my first attempt at writing something in go. Some areas that are marked with `TODO` can still be improved but work mostly. If you want to contribute features or improve the code thats great, send a PR and we can discuss.

### Building

Using a recent version of go, building should be straightforward. Either use `go build`, `go run` etc. or use the included Makefile.

### Structure Overview

The `main.go` contains most UI elements which are based around a tview app running on the main routine. It uses a keymap configuration based on the tslocum/cbind library. Apart from that it mostly manages the selection of messages in the current chat as well as displaying the messages and chat list that the session manager sends.

The `messages/session_manager.go` runs a separate go routine to receive messages from the Rhymen/go-whatsapp library which in turn runs the websocket connection to the whatsapp server. The session manager receives the messages from go-whatsapp and the commands from the UI via channels that it drains on its main routine. It then updates the UI accordingly using the UiMessageHandler interface. This ensures "thread safe" management of the connection and data while both UI and network connection run separately.

Session manager is designed "object like", the MessageDatabase in `messages/storage.go` is similar and somewhat linked to the session manager. In theory the session manager could be run multiple times (multiple accounts) or a different implementation of a session manager could connect to a different service like e.g. Telegram.

In `messages/messages.go` most interfaces and data structures for communication are kept.

The `config/settings.go` keeps a singleton `Config` struct with the config that is loaded via the gopkg.in/ini.v1 library when the app starts. This makes it easy to quickly add new configuration items with default values that can be used across the app.
