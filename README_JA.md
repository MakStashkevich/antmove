<div align="center">
  <h1>antmove</h1>
  <p><strong>重いアプリやデータを外付けディスクへ。アプリはこれまで通り使えます。</strong></p>
  <p>Mac本体のストレージ容量に縛られず、もっと自由に。</p>
  <p>macOS専用 · ターミナルCLI · ディスク適性チェック · 移動前プレビュー · 復元に対応</p>
  <p><a href="README.md">🇺🇸 English</a> · <a href="README_JA.md">🇯🇵 日本語</a> · <a href="README_ZH_CN.md">🇨🇳 简体中文</a> · <a href="README_KO.md">🇰🇷 한국어</a> · <a href="README_FR.md">🇫🇷 Français</a> · <a href="README_UK.md">🇺🇦 Українська</a> · <a href="README_RU.md">🇷🇺 Русский</a></p>
  <p><a href="https://github.com/MakStashkevich/antmove/stargazers"><img src="https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat" alt="Stars"></a> <a href="https://github.com/MakStashkevich/antmove/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://github.com/MakStashkevich/antmove"><img src="https://img.shields.io/badge/version-0.1.0-green.svg" alt="Version 0.1.0"></a> <a href="https://x.com/makstashkevich"><img src="https://img.shields.io/badge/X-follow-000000?logo=x&logoColor=white" alt="Follow on X"></a> <a href="https://t.me/maksidea"><img src="https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white" alt="Telegram"></a></p>
</div>

Macの空き容量がなくなる。アプリを入れたいとき、Xcodeを更新したいとき、ゲームを落としたいとき、iPhoneのバックアップを取りたいときに限って。

自分もこれで困りました。内蔵SSDがアプリやそのデータ、開発ツール、バックアップ、写真、動画、音楽でいっぱいになって、普通に作業するのがしんどくなってきたんです。

容量だけのためにMacを買い替えるのは、かなり高い答えに感じました。AI企業が大きなデータセンターを作って、メモリやストレージの話がますます重たくなっている。でも自分に必要だったのは新しいMacではなく、空き場所でした。

そこでantmoveを作りました。対応するアプリやデータを外付けディスクへ移し、コピーを確認して、元のパスにはリンクを残します。アプリはいつもの場所を見に行く。実際のファイルは別のディスクにある。ざっくり言うとそういう仕組みです。

## 仕組みはこれだけ

アプリがいつもの住所に来たら、そこに外付けディスクへの案内を置いておく感じです。

macOSもアプリも、前から使っていたパスをそのまま使います。重たいファイルだけが接続中の外付けディスクにあります。

必要になったら対応ディスクを足していけます。「容量無限」はあくまで言い方です。使うときはディスクが必要だし、antmoveが見つけて確認できるデータだけを移してください。

## antmoveが見つけるもの

- <code>/Applications</code> と <code>~/Applications</code> のアプリ。
- インストール済みアプリと明確に関連付けられる大きなデータフォルダ。
- Xcode/Apple開発データ、Android SDK/AVD、一部の開発ツールデータ。
- macOSが許可すれば、Photosライブラリ、Music/iTunesメディア、Apple TVのメディア/ダウンロード、iPhone/iPadバックアップ。
- 昔のExtender移行記録。

Docker Desktop、OrbStack、Colima、Rancher Desktopのストレージも、メーカーの案内つきで表示されます。でも、antmoveはエンジンの中身を直接移しません。VMやデータベースを普通のフォルダのように動かすと、壊れることがあるからです。

どのアプリのデータか確信できない大きなフォルダは、勝手に決めつけず、そのまま表示します。

## インストール

Apple Siliconは<code>arm64</code>、Intel Macは<code>amd64</code>のアーカイブです。使っているMacに合うアーカイブを[GitHub Releases](https://github.com/MakStashkevich/antmove/releases)からダウンロードしてください。

解凍して一度起動してください。

~~~sh
cd ~/Downloads/antmove
./antmove
~~~

初回起動時に<code>antmove</code>と短いコマンド<code>am</code>を<code>~/.local/bin</code>に入れます。ここが<code>PATH</code>にない場合はシェル設定へ追加するので、その後Terminalを開き直してください。

インストールせず、展開したフォルダから毎回起動することもできます。

~~~sh
./antmove
~~~

手動でインストールまたは修復する場合:

~~~sh
./install-local.sh
~~~

### Bashスクリプトで入れる

<code>install.sh</code>はmacOSとCPUアーキテクチャを確認し、合う最新リリースをダウンロードしてインストールします。ファイルのスキャンや移動はしません。普通のBashスクリプトなので、実行前にリンク先で内容を確認できます。

~~~sh
curl -fsSL https://raw.githubusercontent.com/MakStashkevich/antmove/main/install.sh | bash
~~~


### Homebrewでインストール

~~~sh
brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git
brew install antmove
~~~

更新するとき:

~~~sh
brew upgrade antmove
~~~

Homebrewはソースからビルドし、<code>antmove</code>と<code>am</code>をインストールします。ビルド用のGoも自動で入ります。

### 自分でビルドする

ソースからのビルドにはGo 1.24以降が必要です。

~~~sh
git clone https://github.com/MakStashkevich/antmove.git
cd antmove
make build
./bin/antmove install
~~~

リポジトリURLは公開後に追加します。配布バイナリの実行にはGo、Swift、Xcodeは要りません。

### AIに頼む

Codex、Claude Code、Cursorなどで使えるantmoveスキルもあります。次のコマンドで追加できます。

~~~sh
npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y
~~~

CodexとClaude Code用のプラグインもあります。ファイルは<code>.agents/plugins</code>と<code>plugins/antmove</code>、追加方法は<code>plugins/antmove/README.md</code>です。

AIには、例えばこう頼めます。

> 公式の手順でantmoveをインストールして。コマンドを説明して、ユーティリティだけを入れ、am --versionで確認して。頼むまでは何もスキャンしたり移動したりしないで。

インストールを頼んだことは、データ移動を許可したことにはなりません。実際に動かす前に<code>--dry-run</code>の計画を出してもらい、何が変わるか見てから決めてください。

## まずここから

メニューを開く:

~~~sh
am
~~~

矢印キーで移動、Enterで選択、Spaceで項目にチェック。キー操作の案内はいつも画面の下に出ています。

ターミナルの表示は英語です。ドキュメントは7言語で用意しました。

まずは見つかったものを見るだけ:

~~~sh
am scan
~~~

次に、使えそうなディスクを確認:

~~~sh
am disks
~~~

見るのは形式、速度、接続方式、空き容量です。SSD上のAPFSがいちばんおすすめ。HFS+やHDD上のAPFSも使えますが、アプリやゲームは遅くなります。ExFAT、FAT、未知の形式ではmacOSのメタデータやアプリ互換性に問題が出ることがあるので警告します。

ネットワーク、システム、読み取り専用、今見つからないディスクは転送先にできません。

速度を測るときは256 MiBの一時ファイルを書いて読み、最後に削除します。

~~~sh
am disks --test "/Volumes/Work SSD"
~~~

読み取り速度の目安:

| 用途 | 十分 | 良い | とても良い |
| --- | ---: | ---: | ---: |
| ゲーム | 150 MB/s | 350 MB/s | 700 MB/s |
| オフィスアプリ | 30 MB/s | 75 MB/s | 150 MB/s |
| メッセンジャー | 20 MB/s | 50 MB/s | 100 MB/s |

## 先に見て、それから移す

自分ならこの順番にします。

1. 外付けディスクを接続して<code>am disks</code>で確認。
2. <code>am scan</code>で名前、サイズ、状態、メモを確認。
3. 対象アプリを終了して、バックアップや同期が終わるまで待つ。
4. メニューの **Move data** を選び、項目とディスクを指定し、警告を読んで確認。
5. 移したデータを使う間はディスクを接続しておく。

antmoveはアプリやコンテナエンジンを勝手に終了しません。起動中なら自分で閉じて、もう一度スキャンしてください。

<code>am scan</code>のIDでプレビューできます。

~~~sh
am move --disk "/Volumes/Work SSD" --dry-run 71a29f5a1cd0
~~~

何もコピーせず、変更もありません。計画を読んで大丈夫そうなら実行します。

~~~sh
am move --disk "/Volumes/Work SSD" --yes 71a29f5a1cd0
~~~

おすすめディスクへの非対話転送では<code>--yes</code>を使います。注意が出るディスクでは二回確認が必要です。

~~~sh
am move --disk "/Volumes/Shared" --confirm-caution --confirm-caution 71a29f5a1cd0
~~~

警告を読まずに確認しないでください。ディスクが遅いとゲームも重くなるし、ファイルシステムによってはmacOSのメタデータが保たれないことがあります。二回確認しても、そのディスクがおすすめになるわけではありません。

antmoveは空き容量を確認し、macOSの<code>ditto</code>で一時フォルダにコピーし、ハッシュとフォルダ内容を比べます。確認してから元のパスをシンボリックリンクに置き換えます。操作記録は<code>~/Library/Application Support/antmove</code>です。

## 写真、音楽、映画、バックアップのこと

- **Photos:** <code>.photoslibrary</code>を移す前にPhotosを終了。AppleはAPFSまたはMac OS Extendedを推奨しています。移動後は新しい場所から開き、iCloud Photosなら必要に応じてシステムフォトライブラリに指定してください。
- **Music:** Musicを終了してから移動。今後の読み込み先は **Music → Settings → Files → Media Location** で確認してください。antmoveはMusicの設定を変えません。
- **Apple TV:** TVを終了。ダウンロードした映画にはDRMがあることがあります。移動後に再生を確認し、見るときはディスクを接続してください。
- **iPhone/iPadバックアップ:** 標準の場所は<code>~/Library/Application Support/MobileSync/Backup</code>です。Finderのバックアップや同期が終わってから移動。アクセス拒否なら、<code>am</code>を動かすTerminalなどにFull Disk Accessを付け、開き直して再スキャンしてください。
- **Docker:** エンジンが用意している方法を使ってください。VMやデータベースのファイルを普通のフォルダのように動かすのはやめたほうがいいです。

Appleの説明: [Photosライブラリを移動](https://support.apple.com/en-ie/108345) · [Musicのファイル設定](https://support.apple.com/en-ca/guide/music/mus69248042d/mac) · [Apple TVのファイル設定](https://support.apple.com/en-gb/guide/tvapp-mac/tv0f5accea/mac) · [iPhoneバックアップを探す](https://support.apple.com/en-la/108809)。

## 元に戻したいとき

最初に復元プランを見る:

~~~sh
am restore --dry-run 71a29f5a1cd0
~~~

大丈夫なら復元:

~~~sh
am restore --yes 71a29f5a1cd0
~~~

操作履歴とディスクの状態は<code>am status</code>で確認できます。途中で止まったらディスクを接続して<code>am recover</code>を実行します。

antmoveを消す場合は二回確認があります。

~~~sh
am uninstall
~~~

アンインストールしてもデータは戻りません。先に<code>am restore</code>を使うか、ディスクとリンクをそのまま残してください。

## コマンド一覧

| コマンド | 内容 |
| --- | --- |
| <code>am</code> | メニューを開く |
| <code>am install</code> | コマンドをインストール/修復 |
| <code>am uninstall</code> | 二回確認して削除 |
| <code>am scan</code> | 候補と状態を表示 |
| <code>am disks</code> | 利用できるディスクを表示 |
| <code>am disks --test PATH</code> | 読み書き速度を測る |
| <code>am move --disk PATH [ID…]</code> | 選んだ項目を移動 |
| <code>am restore [ID…]</code> | 元の場所へ戻す |
| <code>am status</code> | 操作記録とディスク状態 |
| <code>am recover</code> | 中断した操作を復旧 |
| <code>am migrate --dry-run</code> | Extender記録の取り込みをプレビュー |
| <code>am migrate</code> | Extender記録を取り込む |
| <code>am --help</code> | ヘルプ |
| <code>am --version</code> | バージョンとシステム情報 |

## 自分でビルドしたい場合

Go 1.24以降が必要です。macOSなら、手元のMac向けにも、両方のアーキテクチャにもビルドできます。

~~~sh
make build
make build-arm64
make build-amd64
make release
make test
make check
~~~

アーカイブは<code>dist/</code>にできます。リリースバイナリはまだ署名も公証もしていないため、Gatekeeperが **System Settings → Privacy & Security** で許可を求めることがあります。

## ライセンス

antmoveは[MIT License](LICENSE)で公開しています。

