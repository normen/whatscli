# whatscli

A command line interface for whatsapp, based on [go-whatsapp](https://github.com/Rhymen/go-whatsapp) and [tview](https://github.com/rivo/tview)

```
┌─────────────────────────────────────────────────────────────────────────┐
│ WhatsCLI v0.4.2  Help: /name NewName | /addname 123456 NewName | /quit |│
├──────────────────────────────┬──────────────────────────────────────────┤
│Contacts                      │(03-14-12 22:59:00) Me: Hey, whatscli here│
│├──Peter                      │(03-14-12 23:00:00) Peter: Cool 😀        │
│├──Paul                       │                                          │
│└──Mary                       │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              │                                          │
│                              ├──────────────────────────────────────────┤
│                              │Yeah, love the shell!                     │
└──────────────────────────────┴──────────────────────────────────────────┘
```

## Features

Things that work.

- Connects through the Web App API without a browser
- Allows sending and receiving WhatsApp messages in a command line app
- Allows downloading and opening image/video/audio/document attachments
- Uses QR code for simple setup
- Binaries for Windows, Mac, Linux and RaspBerry Pi

### Caveats

This is a WIP. Heres some things you might expect to work that don't. Plus some other things I should mention.

- Only shows existing chats
- Only fetches a few old messages
- No incoming message notification / count
- No proper connection drop handling
- Not configurable at all
- Leaves its config files in your home folder
- FaceBook obviously doesn't endorse or like these kinds of apps and they're likely to break when FaceBook changes stuff in their web app

## Installation / Usage

How to get it running and how to use it

- Download a release
- Put the binary in your PATH (optional)
- Run with `whatscli` (or double-click)
- Scan the QR code with WhatsApp on your phone (maybe resize shell)

