# Obxod :)

Bypasses DPI blocking of TLS on Windows. Intercepts outgoing packets through
WinDivert, cuts the hello where the site name sits and puts a made up name of the
very same size in its place. The inspector reads the packets in the order they
arrive and matches the made up name; the server puts the stream back together by
sequence number and gets the real one, because the real name follows and lands on
top of the fake.

## Build

    go build ./cmd/obxod

## Run

Needs Windows, administrator privileges, and two files from WinDivert 2.2 sitting
next to the executable:

    WinDivert.dll
    WinDivert64.sys

Both come from the x64 folder of the official release at
https://reqrypt.org/windivert.html and are signed by its author. The driver
installs itself the first time the program opens it, which is what the
administrator privileges are for. To remove it, delete both files and reboot.

Nothing goes out until `-wet` is given. Without it the program only reports what
it would have sent, which is the safe way to see whether a site is recognised at
all.

## Rules

One rule per site, repeated as many times as there are sites. A bare domain
covers its subdomains, so `discord.com` also covers `updates.discord.com`.

    host=way,way,way

Run `obxod.exe -h` for the ways and what each one does.

A site usually lives on more than one domain, and a missing one is invisible: the
program says nothing about traffic no rule covers. Discord, for instance, serves
its own content from `discordapp.com`, which `discord.com` does not cover.

## What works

Measured against one provider, so read it as a starting point rather than a
setting. Keep the rules in a file and edit them as you find more names:

    # rules.txt
    discord.com=hostfake:mail.ru,ts
    discord.gg=hostfake:mail.ru,ts
    discordapp.com=hostfake:mail.ru,ts
    discordapp.net=hostfake:mail.ru,ts
    discordcdn.com=hostfake:mail.ru,ts
    discord.media=hostfake:mail.ru,ts
    youtube.com=hostfake:mail.ru,ts
    googlevideo.com=hostfake:mail.ru,ts
    ytimg.com=hostfake:mail.ru,ts
    ggpht.com=hostfake:mail.ru,ts
    x.com=hostfake:mail.ru,ts

    obxod.exe -wet -noquic -rules rules.txt

The same rule carried Discord, YouTube and X whole. Browsers reach for HTTP/3
first, and nothing here touches datagrams, so `-noquic` is what makes them fall
back to tcp where the rules apply. It drops every outgoing quic datagram, not
only the ones for these sites.

A site lives on more names than anyone remembers, and one missing name is
invisible: traffic no rule covers goes out untouched and nothing is said about it.
`-seen` names each one once, which is how the list above was written.

    obxod.exe -wet -noquic -seen -rules rules.txt

Three things about those rules took a week to find, and none of them is obvious.

`hostfake` swaps the name inside the stream. Sending a whole forged hello ahead of
the real one instead, which is the obvious move, gets the handshake through and
then stalls around a fifth of the page: the inspector throws the forged copy away
along with the server and reads the real name off the packets that follow.

`ts` and nothing else. The fake has to be spoiled so the server drops it, but the
inspector still has to read it. Moving the timestamp back leaves the sequence
number where it belongs, so the fake stays in the stream. Moving the sequence
number instead puts it outside the window and nobody reads it, which is the same
as not sending it.

The name matters as much as the method. `hostfake:mail.ru` loads the whole page;
the made up name the program picks on its own gets a third of it. The inspector
reads the name and judges it.

## What a failure looks like

A page that never arrives and a page that arrives saying no are different
problems, and only the first one is this program's.

    This service is not available in your region.

That is the site answering. The connection got there, the name went through, and
the service turned it down on the address it came from. Nothing here can change
which address that is.

An empty answer after a long wait is the other kind, and that is the one worth
chasing with rules.

## Checking

The program has no diagnostics of its own, and whether a site loads is too coarse
to tell one rule from another. Ask for a page large enough to need more than the
first few packets, and watch the bytes rather than the status:

    curl -s -m 25 -o NUL -w "%{http_code} %{size_download} bytes %{time_total}s" https://discord.com/app

A handshake that gets through but a stream that stops early looks like a status of
200 with a fraction of the bytes.

What a run means depends on what is blocked at that moment, and that changes.
Measure the same thing twice with the rule on and off, one right after the other,
and compare the neighbours. A single run on its own says nothing.

![](assets/smile.png)
