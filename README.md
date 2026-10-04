# Webdownload

EleBBS protocol replacement from Shurato. Instead of sending tagged files over a transfer protocol, this door copies them into a web directory and shows the caller a download URL.

It is a standalone console program. It reads `DOOR32.SYS`, talks over the inherited telnet/socket session, and leaves ANSI color codes intact.

## Downloads

Binaries are on the [Releases](https://github.com/martykazmaier/Webdownload/releases/tag/webdownload) page:

| File | Platform |
| --- | --- |
| `webdownload.exe` | Windows 32-bit |
| `webdownload-linux-386` | Linux 32-bit |
| `webdownload-linux-amd64` | Linux 64-bit |
| `webdownload-linux-arm64` | Linux ARM 64-bit |
| `webdown.zip` | All four, plus `readme.txt` and `file_id.diz` |

## What it does

1. Reads `DOOR32.SYS` in the current directory (socket handle, baud, alias, time left).
2. Copies tagged files into `webdir/webfiles/<user>/<16 hex chars>/`.
3. Writes `index.html` in that folder.
4. Shows the public URL over the Door32 session.
5. Waits for **Escape**, **Ctrl-X**, or **Ctrl-C**.
6. Appends a DSZ-style transfer log so EleBBS can count the downloads.

Public URL:

```text
https://bbs.example/webfiles/<user>/<subdir>/
```

Do not keep other content under `webfiles`. Cleanup deletes expired folders there.

## Usage

```text
webdownload [options] <weburl> <webdir> <username> [ctlfile]
webdownload -cleanup <webdir> [username]
```

| Argument | Meaning |
| --- | --- |
| `weburl` | Public site, e.g. `https://shsbbs.net`. A host without a scheme gets `https://`. |
| `webdir` | Filesystem root of that site. Files land under `webdir/webfiles/`. |
| `username` | Usually `%handle`. |
| `ctlfile` | Optional. Starts with `@` = list file (DSZ.CTL). No `@` = one file to copy. If omitted, `DSZ.CTL` beside `DOOR32.SYS` is used as a list. |

### Windows (EleBBS)

Work directory = the node directory that contains `DOOR32.SYS`.

```text
C:\path\webdownload.exe https://bbs.example C:\WWW %handle @C:\NODE\DSZ.CTL
```

One file instead of a list:

```text
C:\path\webdownload.exe https://bbs.example C:\WWW %handle C:\FILES\GAME.ZIP
```

### Linux

```text
./webdownload https://bbs.example /var/www/bbs %handle @/node/dsz.ctl
```

Make the binary executable (`chmod +x`) and use the build that matches the machine.

### Options

| Option | Meaning |
| --- | --- |
| `-door32 path` | Path to `DOOR32.SYS` if it is not in the current directory. |
| `-local` | Use stdin/stdout instead of the Door32 socket. |
| `-ws` | Frame ANSI as gorilla websocket messages. Default I/O is raw TCP on the Door32 handle. |
| `-cleanup` | Expire download dirs under `webdir/webfiles` older than 24 hours, then exit. |

## Cleanup

Run from a daily event so old download folders do not fill the web volume:

```text
webdownload -cleanup C:\WWW
webdownload -cleanup C:\WWW %handle
```

Omit the username to clean every account under `webfiles`.

## Limits

- Tagged files totaling more than **5 GB** are refused before copy.
- The destination volume must have at least **20 GB** free.

## EleBBS notes

- The fourth argument follows DSZ: `@DSZ.CTL` is a list; a path without `@` is one file.
- EleBBS often passes `@C:\NODE\DSZ.CTL`. The `@` is stripped to find the list file on disk.
- After a successful copy, a DSZ log is written (`dszlog` / `DSZ.LOG`) so `ProcessTransferLog` can credit downloads.
- The Linux build uses the inherited Door32 file descriptor and does not close the telnet socket. EleBBS still owns the session.
