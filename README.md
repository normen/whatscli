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

