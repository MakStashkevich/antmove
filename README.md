# antmove — i don't want to replace my mac just because the drive is full
> move the heavy stuff out. keep working like before.

[🇺🇸 English](README.md) · [🇯🇵 日本語](README_JA.md) · [🇨🇳 简体中文](README_ZH_CN.md) · [🇰🇷 한국어](README_KO.md) · [🇫🇷 Français](README_FR.md) · [🇺🇦 Українська](README_UK.md) · [🇷🇺 Русский](README_RU.md)

![Stars](https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE) [![Version](https://img.shields.io/badge/version-0.1.0-green.svg)](#)

author: [Maksim Stashkevich](https://stashkevich.com) · [GitHub](https://github.com/MakStashkevich/antmove) · [x](https://x.com/makstashkevich) · [telegram](https://t.me/maksidea)


so, my mac ran out of space. not someday in theory — right when i needed to install an app, update Xcode, download a game, or keep another iPhone backup.

i ran into this myself. the internal SSD filled up with apps and their data, developer tools, backups, photos, videos, and music. at some point all of it started getting in the way of simply doing the work.

buying a new mac just because the old one had a small drive felt like a pretty expensive answer. and while AI giants are building data centers and buying up resources, storage prices feel even more painful. i didn't need a new computer. i needed more room.

so i made antmove. it moves supported apps and data to an external disk, checks the copy, and leaves a link at the old path. the app keeps going to the same place it always did. the files just live on another disk now.

## what it actually does

think of an app showing up at its usual address and finding a redirect there.

macOS and the app keep using the familiar path. the heavy files are physically stored on the connected external disk. that's the whole trick.

you can add compatible disks when you need more room and keep only the everyday essentials on the internal drive. “infinite storage” is an image, of course. the disk still needs to be connected, and the data has to be something antmove knows how to identify and verify.

## what antmove can find

- apps in <code>/Applications</code> and <code>~/Applications</code>;
- large data folders that can be clearly matched to an installed app;
- Xcode and Apple developer data, Android SDK and AVD, and some developer tool data;
- Apple Photos libraries, Apple Music/iTunes media, Apple TV media and downloads, and iPhone/iPad backups when macOS allows access;
- old Extender transfer records, so they can be imported into antmove's state and journal.

Docker Desktop, OrbStack, Colima, and Rancher Desktop storage can appear with instructions from their makers. antmove doesn't move those engine stores directly. They contain virtual machines and databases, and dragging a folder around can break the engine.

If antmove can't confidently tell which app owns a large folder, it leaves it unassigned. better not to guess.

## how to install

Apple Silicon needs the <code>arm64</code> archive; Intel needs <code>amd64</code>. download the archive for your Mac from [GitHub Releases](https://github.com/MakStashkevich/antmove/releases).

unpack the archive and launch it once:

~~~sh
cd ~/Downloads/antmove
./antmove
~~~

on first launch, antmove installs <code>antmove</code> and the short command <code>am</code> in <code>~/.local/bin</code>. if that folder isn't on <code>PATH</code>, the installer adds it to your shell settings. then reopen Terminal.

you can also skip installation and launch it from the extracted folder:

~~~sh
./antmove
~~~

to install or repair the commands explicitly:

~~~sh
./install.sh
~~~

### use the Bash installer

the project includes <code>install-online.sh</code>. it checks macOS and your processor type, downloads the latest matching release to a temporary folder, and runs the installer from the archive. it doesn't scan or move your files.

download the script and read it before running:

~~~sh
curl -fL -o install-online.sh https://raw.githubusercontent.com/MakStashkevich/antmove/main/install-online.sh
less install-online.sh
bash install-online.sh
~~~


i'd rather you read the script first. piping something you haven't seen through <code>curl | bash</code> is a habit i wouldn't recommend.

### build it yourself

building from source needs Go 1.24 or newer:

~~~sh
git clone https://github.com/MakStashkevich/antmove.git
cd antmove
make build
./bin/antmove install
~~~

prebuilt binaries don't need Go, Swift, or Xcode.

### ask an AI assistant

the repository includes a skill for Codex, Claude Code, Cursor, and similar assistants. add it like this:

~~~sh
npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y
~~~

Codex and Claude Code plugins are also included. their files are in <code>.agents/plugins</code> and <code>plugins/antmove</code>; setup commands are in <code>plugins/antmove/README.md</code>.

you can ask an assistant in plain language:

> install antmove from the official instructions. explain the commands, install only the utility, and check it with am --version. don't scan or move anything unless i ask.

and this matters: asking to install antmove doesn't mean it can move your files. first it should show a <code>--dry-run</code> plan and explain what will change. then you decide.

## where to start

run:

~~~sh
am
~~~

the menu opens. use the arrow keys to move, Enter to choose, and Space to mark items. the keyboard hints stay in the footer.

the terminal interface is in English. the docs are available in seven languages.

first, just see what antmove found:

~~~sh
am scan
~~~

then see which disks are suitable:

~~~sh
am disks
~~~

what matters is the format, speed, connection, and free space. APFS on an SSD is the preferred option. HFS+ and APFS on a hard drive can work, but apps and games will be slower. ExFAT, FAT, and unknown formats come with a warning: macOS metadata or app compatibility may suffer.

network disks, system volumes, read-only disks, and volumes that aren't available right now can't be destinations.

if you want to measure speed, antmove writes a temporary 256 MiB file, reads it, and deletes it:

~~~sh
am disks --test "/Volumes/Work SSD"
~~~

these are the read-speed guideposts:

| workload | suitable | good | excellent |
| --- | ---: | ---: | ---: |
| games | 150 MB/s | 350 MB/s | 700 MB/s |
| office apps | 30 MB/s | 75 MB/s | 150 MB/s |
| messengers | 20 MB/s | 50 MB/s | 100 MB/s |

## look first, move second

this is the order i'd use:

1. connect the external disk and check it with <code>am disks</code>;
2. run <code>am scan</code> and look at the name, size, status, and note;
3. quit the related app and wait for any backup or sync to finish;
4. choose **Move data** in the menu, mark the items, choose a disk, read the warning, and confirm;
5. leave the disk connected while using the moved data.

antmove won't close apps or container engines for you. if something is open, close it yourself and scan again.

you can preview a transfer using the ID from <code>am scan</code>:

~~~sh
am move --disk "/Volumes/Work SSD" --dry-run 71a29f5a1cd0
~~~

that command doesn't copy or change anything. if the plan looks right, run the actual move:

~~~sh
am move --disk "/Volumes/Work SSD" --yes 71a29f5a1cd0
~~~

<code>--yes</code> is for a non-interactive move to a recommended disk. a disk with a caution warning needs two confirmations:

~~~sh
am move --disk "/Volumes/Shared" --confirm-caution --confirm-caution 71a29f5a1cd0
~~~

don't confirm automatically. read why the disk got that rating first. a slow disk can make games feel awful, and a filesystem may fail to preserve some macOS metadata. two confirmations only mean “i understand the risk”; they don't turn the disk into a good one.

antmove checks free space, copies with macOS <code>ditto</code> into a temporary folder, compares hashes and directory contents, and only then leaves a link to the new location. the operation journal lives in <code>~/Library/Application Support/antmove</code>.

## a few notes about photos, music, movies, and backups

- **Photos.** quit Photos before moving a <code>.photoslibrary</code>. Apple recommends APFS or Mac OS Extended. Open the library from its new location; if you use iCloud Photos, select it as the System Photo Library if needed. Keep the disk connected.
- **Music.** quit Music. After the move, check <code>Music → Settings → Files → Media Location</code>, especially if you'll add new files. antmove doesn't change Music settings.
- **Apple TV.** quit TV too. Downloaded movies may be DRM-protected. Check playback after the move and keep the disk connected while watching. The media folder is configured in <code>TV → Settings → Files</code>.
- **iPhone/iPad backups.** the standard location is <code>~/Library/Application Support/MobileSync/Backup</code>. Wait for Finder to finish backup or sync. If macOS denies access, give Full Disk Access to the terminal app running <code>am</code>, reopen it, and run <code>am scan</code> again.
- **Docker.** use the engine's supported settings. i wouldn't drag VM or database files around like ordinary folders.

Apple's guides: [move a Photos library](https://support.apple.com/en-ie/108345), [Music file settings](https://support.apple.com/en-ca/guide/music/mus69248042d/mac), [Apple TV file settings](https://support.apple.com/en-gb/guide/tvapp-mac/tv0f5accea/mac), [find iPhone backups](https://support.apple.com/en-la/108809).

## if you need to put things back

first preview what will be restored:

~~~sh
am restore --dry-run 71a29f5a1cd0
~~~

if the plan looks right, restore the data:

~~~sh
am restore --yes 71a29f5a1cd0
~~~

check the journal and disk connection with:

~~~sh
am status
~~~

if a transfer got interrupted, connect the disk and run:

~~~sh
am recover
~~~

if you want to remove antmove, the command asks you to confirm twice:

~~~sh
am uninstall
~~~

uninstalling the utility doesn't bring moved data back. run <code>am restore</code> first, or keep the disk connected with the links in place.

## all commands

| command | what it does |
| --- | --- |
| <code>am</code> | open the menu |
| <code>am install</code> | install or repair the commands |
| <code>am uninstall</code> | remove antmove after two confirmations |
| <code>am scan</code> | find candidates and show their status |
| <code>am disks</code> | show disks and suitability |
| <code>am disks --test PATH</code> | measure read and write speed |
| <code>am move --disk PATH [ID…]</code> | move selected items |
| <code>am restore [ID…]</code> | restore data to its original path |
| <code>am status</code> | show the journal and disk availability |
| <code>am recover</code> | continue recovery after an interruption |
| <code>am migrate --dry-run</code> | preview the Extender record import |
| <code>am migrate</code> | import old Extender records |
| <code>am --help</code> | show help |
| <code>am --version</code> | show version and system info |

## if you want to build it

you'll need Go 1.24 or newer. on macOS you can build for your machine, both architectures, or package release archives:

~~~sh
make build
make build-arm64
make build-amd64
make release
make test
make check
~~~

archives go into <code>dist/</code>. <code>make test</code> and <code>make check</code> run the Go tests.

release binaries aren't signed or notarized yet. macOS may ask you to allow them in **System Settings → Privacy & Security**.

## license

antmove is released under the [MIT License](LICENSE).

