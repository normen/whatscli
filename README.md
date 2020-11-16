# whatscli

A command line interface for whatsapp, based on go-whatsapp and tview

```
┌────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ WhatsCLI v0.1.0  Help: /name  = name contact | /quit = exit app | /load = reload contacts | <Tab> =│
├──────────────────────────────┬─────────────────────────────────────────────────────────────────────┤
│Contacts                      │                                                                     │
│├──Normen                     │(03-22-13 21:18:24) Daniel: Sachste bescheid wenn der kram vorbei ist│
│├──Lilou                      │? :)                                                                 │
│├──Boris                      │(03-22-13 21:37:29) Ich: Jo                                          │
│├──Malte                      │(03-22-13 22:02:13) Daniel: Mensch das geht ja lang..                │
│├──Daniel                     │(03-22-13 22:02:53) Ich: Nu is schulz                                │
│├──Seb                        │(03-22-13 22:03:08) Daniel: Jo bis gleich !                          │
│├──Bettina                    │(04-07-14 18:06:15) Daniel: Hey wie schauts :)                       │
│├──Elternbeirat               │(04-07-14 18:07:56) Ich: Ich komme laut Navigationssystem um 19:40   │
│├──4911758758720-1565273421@g.│Uhr an                                                               │
│├──491111250015@s.whatsapp.net│(04-07-14 18:08:33) Daniel: Sauber, ruf ma so ca 10 min vorher durch │
│├──4911758758714-1537904683@g.│dann bin ich da :)                                                   │
│├──491114171906-1448397341@g.u│(04-07-14 19:24:21) Ich: Bin jetzt in Bremen. Circa 10 Minuten       │
│├──491192855547-1561396191@g.u│(04-07-14 19:24:45) Daniel: Ok mach mich los                         │
│├──491152456088@s.whatsapp.net│(07-27-14 19:14:19) Ich: Moin do. Sag' mal bist Du morgen um fünf in │
│├──491107382606-1364411990@g.u│Bremen? Ich bräuchte jemanden um das Mischpult etc. wieder in einen  │
│├──491111250017-1603197565@g.u│LKW zu laden..                                                       │
│├──491131942996@s.whatsapp.net│(07-27-14 19:15:22) Daniel: Ich bin unterwegs, sorry                 │
│├──491122978981@s.whatsapp.net│(07-27-14 19:15:52) Ich: Kein Ding, danke!                           │
│├──491192855528@s.whatsapp.net│(07-27-14 19:24:50) Daniel: Jou bin quasi noch im breminale stress :)│
│├──491154447429@s.whatsapp.net│(07-27-14 19:25:12) Ich: Na dann noch viel Spass :)                  │
│├──491132457405-1526385826@g.u│(07-27-14 19:25:34) Daniel: Ja danke, bin froh wenns vorbei is ;)    │
│├──491103663035@s.whatsapp.net│(07-27-14 19:26:12) Ich: Augen zu und durch, Lock'n'Loll             │
│├──491113075747@s.whatsapp.net│(11-15-20 15:27:06) Ich: testjj                                      │
│├──491147048885@s.whatsapp.net├─────────────────────────────────────────────────────────────────────┤
│├──491124146101@s.whatsapp.net│                                                                     │
└──────────────────────────────┴─────────────────────────────────────────────────────────────────────┘
```

## Features

Things that work.

- Allows sending and receiving WhatsApp messages in a CLI interface
- Connects through Web App API without browser
- Uses QR code for simple setup

### Caveats

This is a WIP. Heres some things you might expect to work that don't. Plus some other things I should mention.

- Only lists contacts that have been messaged on phone
- Only fetches a few messages for last contacted
- To display names they have to be entered through the `/name TheName` command for each contact
- Doesn't support naming people in groups
- No support for images, videos, documents etc
- No incoming message notification / count
- Not configurable at all
- Leaves its config files in your home folder
- FaceBook obviously doesn't endorse or like these kinds of apps and they're likely to break when they change stuff in their web app

## Installation / Usage

How to get it running and use it

- Put the binary in your PATH (optional)
- Run with `whatscli` (or double-click)
- Scan QR code with WhatsApp on phone (maybe resize shell)

