<div align="center">
  <h1><img src="src/assets/ant.png" width="48" alt="🐜" align="middle"> antmove</h1>
  <p><strong>Move heavy apps and data to an external drive—and use your apps like before.</strong></p>
  <p>More freedom from your Mac’s built-in storage limits.</p>
  <p>macOS only · Terminal CLI · disk suitability checks · preview before moving · restore when needed</p>
  <p><a href="README.md">🇺🇸 English</a> · <a href="README_JA.md">🇯🇵 日本語</a> · <a href="README_ZH_CN.md">🇨🇳 简体中文</a> · <a href="README_KO.md">🇰🇷 한국어</a> · <a href="README_FR.md">🇫🇷 Français</a> · <a href="README_UK.md">🇺🇦 Українська</a> · <a href="README_RU.md">🇷🇺 Русский</a></p>
  <p><a href="https://github.com/MakStashkevich/antmove/stargazers"><img src="https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat" alt="Stars"></a> <a href="https://github.com/MakStashkevich/antmove/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://github.com/MakStashkevich/antmove/releases/latest"><img src="https://img.shields.io/github/v/release/MakStashkevich/antmove?display_name=tag" alt="Latest release"></a> <a href="https://x.com/makstashkevich"><img src="https://img.shields.io/badge/X-follow-000000?logo=x&logoColor=white" alt="Follow on X"></a> <a href="https://t.me/maksidea"><img src="https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white" alt="Telegram"></a></p>
</div>

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
- Apple Photos libraries, Apple Music media, existing legacy iTunes folders, Apple TV media and downloads, and iPhone/iPad backups when macOS allows access;
- old Extender transfer records, so they can be imported into antmove's state and journal.

Docker Desktop, OrbStack, Colima, and Rancher Desktop storage can appear with instructions from their makers. antmove doesn't move those engine stores directly. They contain virtual machines and databases, and dragging a folder around can break the engine.

If antmove can't confidently tell which app owns a large folder, it leaves it unassigned. better not to guess.

## how to install

the archive name says which Mac it fits: <code>apple-silicon</code> is for M-series Macs, and <code>intel</code> is for Intel Macs. download the matching archive from [GitHub Releases](https://github.com/MakStashkevich/antmove/releases).

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
./install-local.sh
~~~

### use the Bash installer

the project includes <code>install.sh</code>. it checks macOS and your processor type, downloads the latest matching release to a temporary folder, and installs the CLI. it doesn't scan or move your files.

install with one command:

~~~sh
curl -fsSL https://raw.githubusercontent.com/MakStashkevich/antmove/main/install.sh | bash
~~~


the script is plain Bash; you can read it at the same URL before running it. It only downloads antmove and runs its installer. it doesn't scan or move your files.

### install with Homebrew

~~~sh
brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git
brew install antmove
~~~

To update later:

~~~sh
brew upgrade antmove
~~~

Homebrew builds antmove from source and installs both <code>antmove</code> and <code>am</code>. Go is installed automatically as a build dependency.

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

if a related app is open, the interactive flow asks separately before antmove sends a graceful quit request. the CLI equivalent is <code>--close-apps</code>. macOS may ask the terminal for Automation permission. antmove won't force-quit apps or stop container engines and background services.

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
- **Apple TV.** quit TV too. Antmove reads the Media folder path configured in <code>TV → Settings → Files</code> (the default is <code>~/Movies/TV/Media.localized</code>) instead of moving its enclosing library folder. Apple does not document a stable filesystem path for protected downloads, so Antmove only lists a known download-cache path if it exists. Downloaded movies may be DRM-protected. Check playback after the move and keep the disk connected while watching.
- **iPhone/iPad backups.** the standard location is <code>~/Library/Application Support/MobileSync/Backup</code>. Wait for Finder to finish backup or sync. If macOS denies access, give Full Disk Access to the terminal app running <code>am</code>, reopen it, and run <code>am scan</code> again.
- **Closing active apps.** in the interactive move flow, Antmove asks separately before sending an active related app a graceful quit request. For the CLI, add <code>--close-apps</code> (for example, <code>am move --disk /Volumes/WorkSSD --yes --close-apps ID</code>). macOS may ask the terminal app for Automation permission in System Settings → Privacy &amp; Security → Automation. Antmove never force-quits or discards unsaved work, and never stops backup, sync, or container services.
- **Empty or not-yet-created folders.** <code>am scan</code> also lists standard Apple Music, Photos, Apple TV, and iPhone/iPad backup locations when they are empty or missing. A <code>0 B</code> item can be moved too: after confirmation, antmove creates the source folder if needed and links it to the external disk. Future writes through that path can then land on the external disk. Quit the related app and finish any backup or sync first. For Music, check that Media Location points to the linked folder. Photos libraries are managed packages; create a new one with Photos’ Option-open → Create New flow. If Photos cannot initialize the linked placeholder, create the library directly on the external disk and select it in Photos. Legacy iTunes paths appear only if those folders already exist.
- **Docker.** use the engine's supported settings. i wouldn't drag VM or database files around like ordinary folders.

Apple's guides: [move a Photos library](https://support.apple.com/en-ie/108345), [Music file settings](https://support.apple.com/en-ca/guide/music/mus69248042d/mac), [Apple TV file settings](https://support.apple.com/guide/tvapp/files-settings-atvb5536de97/web), [find iPhone backups](https://support.apple.com/en-la/108809).

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

