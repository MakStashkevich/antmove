<div align="center">
  <h1><img src="src/assets/ant.png" width="48" alt="🐜"> antmove</h1>
  <p><strong>把大型应用和数据搬到外置硬盘，应用照常使用。</strong></p>
  <p>不再被 Mac 内置存储空间束缚。</p>
  <p>仅支持 macOS · 终端 CLI · 硬盘适用性检查 · 迁移预览 · 支持恢复</p>
  <p><a href="README.md">🇺🇸 English</a> · <a href="README_JA.md">🇯🇵 日本語</a> · <a href="README_ZH_CN.md">🇨🇳 简体中文</a> · <a href="README_KO.md">🇰🇷 한국어</a> · <a href="README_FR.md">🇫🇷 Français</a> · <a href="README_UK.md">🇺🇦 Українська</a> · <a href="README_RU.md">🇷🇺 Русский</a></p>
  <p><a href="https://github.com/MakStashkevich/antmove/stargazers"><img src="https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat" alt="Stars"></a> <a href="https://github.com/MakStashkevich/antmove/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://github.com/MakStashkevich/antmove/releases/latest"><img src="https://img.shields.io/github/v/release/MakStashkevich/antmove?display_name=tag" alt="Latest release"></a> <a href="https://x.com/makstashkevich"><img src="https://img.shields.io/badge/X-follow-000000?logo=x&logoColor=white" alt="Follow on X"></a> <a href="https://t.me/maksidea"><img src="https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white" alt="Telegram"></a></p>
</div>

Mac 的空间总是在最不合适的时候用完：我要装应用、更新 Xcode、下载游戏，或者给 iPhone 留个备份的时候。

我自己就撞上过这个问题。内置 SSD 被应用、应用数据、开发工具、备份、照片、视频和音乐塞满了。后来连正常做事都开始受影响。

只为了硬盘容量就换整台 Mac，我觉得太贵了。现在 AI 公司还在疯狂建数据中心，内存和存储的价格也显得越来越扎眼。对我来说，需要的不是一台新电脑，而是更多空间。

于是我做了 antmove。它把支持的应用和数据搬到外置盘，检查副本，再在原路径留下一个符号链接。应用还是去它熟悉的地址，真正的大文件现在放在另一块盘上。

## 它到底怎么做

可以把它想成：应用来到原来的地址，那里有人把它引到外置盘。

macOS 和应用继续用之前的路径，实际文件存放在已连接的外置盘里。就是这么回事。

空间不够时，可以再加兼容的磁盘。“无限空间”只是个说法，不是字面承诺。用数据时磁盘得连着，而且只应该搬 antmove 能识别、能检查的内容。

## antmove 能找到什么

- <code>/Applications</code> 和 <code>~/Applications</code> 里的应用；
- 能明确匹配到已安装应用的大型数据目录；
- Xcode 和 Apple 开发数据、Android SDK/AVD，以及部分开发工具数据；
- macOS 允许访问时的 Photos 照片图库、Apple Music 媒体、现存的旧 iTunes 文件夹、Apple TV 媒体/下载目录和 iPhone/iPad 备份；
- 旧 Extender 的迁移记录。

Docker Desktop、OrbStack、Colima 和 Rancher Desktop 的数据也可能出现，并附有厂商建议。但 antmove 不会直接移动这些引擎的内部存储。里面有虚拟机和数据库，普通地挪文件夹可能把引擎弄坏。

如果它不能确定一个大文件夹属于哪个应用，就不会替你猜。会单独列出来让你自己看。

## 怎么安装

压缩包名称会标明适用的 Mac：<code>apple-silicon</code> 适用于 M 系列芯片，<code>intel</code> 适用于 Intel 处理器。从 [GitHub Releases](https://github.com/MakStashkevich/antmove/releases) 下载对应的压缩包。

解压后启动一次：

~~~sh
cd ~/Downloads/antmove
./antmove
~~~

第一次启动时，antmove 会把 <code>antmove</code> 和短命令 <code>am</code> 装进 <code>~/.local/bin</code>。如果这个目录不在 <code>PATH</code> 中，安装器会把它加到 shell 配置里；之后重新打开终端就好。

也可以不安装，直接从解压目录启动：

~~~sh
./antmove
~~~

要手动安装或修复命令：

~~~sh
./install-local.sh
~~~

### 用 Bash 脚本安装发行版

<code>install.sh</code> 会识别 macOS 和处理器架构，下载匹配的最新版本并安装。它不会扫描或移动你的文件。脚本本身就是普通 Bash，想看可以先打开链接。

~~~sh
curl -fsSL https://raw.githubusercontent.com/MakStashkevich/antmove/main/install.sh | bash
~~~


### 通过 Homebrew 安装

~~~sh
brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git
brew install antmove
~~~

之后更新：

~~~sh
brew upgrade antmove
~~~

Homebrew 会从源码编译 antmove，并安装 <code>antmove</code> 和 <code>am</code>。Go 会作为编译依赖自动安装。

### 自己编译

从源码编译需要 Go 1.24 或更新版本：

~~~sh
git clone https://github.com/MakStashkevich/antmove.git
cd antmove
make build
./bin/antmove install
~~~

运行预编译版不需要 Go、Swift 或 Xcode。

### 让 AI 助手帮忙

仓库里有适用于 Codex、Claude Code、Cursor 等助手的 antmove skill，可以这样安装：

~~~sh
npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y
~~~

Codex 和 Claude Code 插件也准备好了，在 <code>.agents/plugins</code> 和 <code>plugins/antmove</code>。安装方法在 <code>plugins/antmove/README.md</code>。

你也可以直接对 AI 说：

> 按官方说明安装 antmove。逐条解释命令，只安装工具，用 am --version 检查。在我提出要求前，不要扫描或移动任何文件。

这点很重要：让 AI 安装工具，不等于允许它搬你的数据。真正操作前，先让它展示 <code>--dry-run</code> 计划，说清楚会改什么，再由你决定。

## 第一次打开

运行：

~~~sh
am
~~~

菜单打开后用方向键移动，Enter 选择，Space 标记条目。键盘提示一直在底部。

终端界面现在是英文，文档有七种语言。

先看看找到什么：

~~~sh
am scan
~~~

再看看哪些盘适合：

~~~sh
am disks
~~~

重点看文件系统、速度、连接方式和剩余空间。SSD 上的 APFS 是首选。HFS+ 和机械硬盘上的 APFS 也可能能用，只是会慢些。ExFAT、FAT 和未知格式会显示警告，因为 macOS 元数据或应用兼容性可能出问题。

网络盘、系统卷、只读盘和当前不可用的卷都不能作为目标盘。

要测速度，antmove 会写入并读取一个临时的 256 MiB 文件，然后删除：

~~~sh
am disks --test "/Volumes/Work SSD"
~~~

读取速度参考：

| 用途 | 够用 | 不错 | 很好 |
| --- | ---: | ---: | ---: |
| 游戏 | 150 MB/s | 350 MB/s | 700 MB/s |
| 办公软件 | 30 MB/s | 75 MB/s | 150 MB/s |
| 即时通讯 | 20 MB/s | 50 MB/s | 100 MB/s |

## 先看看，再搬

我会按这个顺序来：

1. 接上外置盘，用 <code>am disks</code> 看看它合不合适；
2. 运行 <code>am scan</code>，看名称、大小、状态和说明；
3. 退出相关应用，等备份或同步结束；
4. 在菜单里选 **Move data**，标记项目、选磁盘、读完警告再确认；
5. 用这些数据时，别把外置盘拔掉。

如果相关应用仍在运行，交互菜单会单独询问是否向它发送正常退出请求；CLI 可用 <code>--close-apps</code>。macOS 可能会要求终端获得自动化权限。Antmove 不会强制退出应用，也不会停止容器引擎或后台服务。

先用 <code>am scan</code> 输出的 ID 看计划：

~~~sh
am move --disk "/Volumes/Work SSD" --dry-run 71a29f5a1cd0
~~~

这个命令不会复制或更改文件。计划没问题，再真正移动：

~~~sh
am move --disk "/Volumes/Work SSD" --yes 71a29f5a1cd0
~~~

推荐磁盘的非交互操作用 <code>--yes</code>。有警告的磁盘要分别确认两次：

~~~sh
am move --disk "/Volumes/Shared" --confirm-caution --confirm-caution 71a29f5a1cd0
~~~

别习惯性按确认。先看清楚磁盘为什么被标警告。盘慢，游戏就可能卡；文件系统不合适，也可能保不住一部分 macOS 元数据。确认两次只说明你理解风险，不会把它变成推荐磁盘。

antmove 会检查剩余空间，用 macOS 的 <code>ditto</code> 复制到临时目录，核对哈希和目录内容，最后才在原路径放符号链接。操作日志在 <code>~/Library/Application Support/antmove</code>。

## 照片、音乐、电影和备份

- **Photos：**移动 <code>.photoslibrary</code> 前先退出 Photos。Apple 推荐 APFS 或 Mac OS Extended。移动后从新位置打开图库；如果使用 iCloud Photos，必要时把它设成系统照片图库。
- **Music：**退出 Music。移动后看看 **Music → Settings → Files → Media Location**，特别是之后还会导入文件的话。antmove 不改 Music 设置。
- **Apple TV：**先退出 TV。下载的电影可能有 DRM。移动后试播一下，观看时保持磁盘连接。
- **Apple TV 路径：**使用 <code>TV → Settings → Files</code> 中设置的 Media folder，默认是 <code>~/Movies/TV/Media.localized</code>。不会移动整个上级文件夹。Apple 没有公布稳定的下载缓存路径，因此只在已存在时显示已知缓存目录。
- **关闭正在运行的应用：**若相关应用仍在运行，Antmove 会单独询问是否发送正常退出请求。CLI 使用 <code>--close-apps</code>。macOS 可能要求终端在“系统设置 → 隐私与安全性 → 自动化”获得控制权限。Antmove 不会强制退出应用、丢弃未保存内容，也不会停止备份、同步或容器服务。
- **iPhone/iPad 备份：**默认位置是 <code>~/Library/Application Support/MobileSync/Backup</code>。等 Finder 备份/同步结束再搬。如果 macOS 拒绝访问，给运行 <code>am</code> 的终端应用 Full Disk Access，重新打开后再扫描。
- **空文件夹或尚未创建的目录：**<code>am scan</code> 也会列出 Music、Photos、Apple TV 和 iPhone/iPad 备份的标准位置，即使它们为空或还不存在。<code>0 B</code> 的项目也能迁移：确认后 antmove 会按需创建原目录，并把它链接到外置磁盘。之后通过这个路径写入的数据就可以直接存到外置盘。操作前先退出相关应用，并等备份或同步结束。Music 要确认 Media Location 指向这个链接目录。Photos 图库是应用管理的专用包；新建图库请按住 Option 打开 Photos，选择 Create New。如果 Photos 无法在空链接位置初始化图库，就直接在外置盘创建，再从 Photos 中选择它。旧 iTunes 路径只会在相应文件夹确实存在时显示。
- **Docker：**走引擎自己的迁移方式。虚拟机或数据库文件别当普通文件夹拖来拖去，我不会这么做。

Apple 说明：[移动 Photos 图库](https://support.apple.com/en-ie/108345) · [Music 文件设置](https://support.apple.com/en-ca/guide/music/mus69248042d/mac) · [Apple TV 文件设置](https://support.apple.com/guide/tvapp/files-settings-atvb5536de97/web) · [查找 iPhone 备份](https://support.apple.com/en-la/108809)。

## 要是想放回去

先预览会恢复什么：

~~~sh
am restore --dry-run 71a29f5a1cd0
~~~

计划看过没问题，再恢复：

~~~sh
am restore --yes 71a29f5a1cd0
~~~

<code>am status</code> 会显示操作记录和磁盘状态。如果传输中断，接上磁盘后运行 <code>am recover</code>。

卸载 antmove 会要求确认两次：

~~~sh
am uninstall
~~~

卸载工具不会自动把数据搬回来。先用 <code>am restore</code>，或者保留磁盘和原路径的链接。

## 命令表

| 命令 | 做什么 |
| --- | --- |
| <code>am</code> | 打开菜单 |
| <code>am install</code> | 安装或修复命令 |
| <code>am uninstall</code> | 两次确认后卸载 |
| <code>am scan</code> | 查找候选并显示状态 |
| <code>am disks</code> | 查看可用磁盘 |
| <code>am disks --test PATH</code> | 测量读写速度 |
| <code>am move --disk PATH [ID…]</code> | 移动选中的项目 |
| <code>am restore [ID…]</code> | 恢复到原路径 |
| <code>am status</code> | 查看日志和磁盘连接状态 |
| <code>am recover</code> | 继续恢复中断的操作 |
| <code>am migrate --dry-run</code> | 预览导入 Extender 记录 |
| <code>am migrate</code> | 导入旧 Extender 记录 |
| <code>am --help</code> | 查看帮助 |
| <code>am --version</code> | 查看版本和系统信息 |

## 想自己编译的话

需要 Go 1.24 或更新版本。在 macOS 上可以编译当前机器、两种架构，或直接打发行包：

~~~sh
make build
make build-arm64
make build-amd64
make release
make test
make check
~~~

压缩包会放在 <code>dist/</code>。当前发行版还没有签名或公证，Gatekeeper 可能会让你到 **System Settings → Privacy & Security** 手动允许启动。

## 许可证

antmove 使用 [MIT License](LICENSE)。

