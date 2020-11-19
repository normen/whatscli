# whatscli

A command line interface for whatsapp, based on [go-whatsapp](https://github.com/Rhymen/go-whatsapp) and [tview](https://github.com/rivo/tview)

![whatscli-screenshot](/doc/screenshot.png?raw=true "WhatsCLI 0.6.5")

## Features

Things that work.

- Connects through the Web App API without a browser
- Allows sending and receiving WhatsApp messages in a command line app
- Allows downloading and opening image/video/audio/document attachments
- Uses QR code for simple setup
- Binaries for Windows, Mac, Linux and RaspBerry Pi

### Caveats

This is a WIP and mainly meant for my personal use. Heres some things you might expect to work that don't. Plus some other things I should mention.

- Only shows existing chats
- Only fetches a few old messages
- No incoming message notification / count
- No proper connection drop handling
- No uploading of images/video/audio/data
- Not configurable at all (except through your terminal settings)
- Leaves its config files in your home folder
- FaceBook obviously doesn't endorse or like these kinds of apps and they're likely to break when FaceBook changes stuff in their web app

## Installation / Usage

How to get it running and how to use it

### Latest Release

Always fresh, always up to date.

- Download a release
- Put the binary in your PATH (optional)
- Run with `whatscli` (or double-click)
- Scan the QR code with WhatsApp on your phone (maybe resize shell)

### Package Managers

Some unofficial ways to install via package managers are supported but the installed version might be out of date.

##### MacOS

Using homebrew:
- `brew install normen/tap/whatscli`

##### Arch Linux

Arch AUR package:
- `git clone https://aur.archlinux.org/whatscli.git`
- `makepkg -si`
- `yay -S whatscli`

