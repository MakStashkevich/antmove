<div align="center">
  <h1>antmove</h1>
  <p><strong>Mac 저장 공간 걱정은 줄이고, 하던 일은 계속하세요.</strong></p>
  <p>용량 큰 앱과 데이터를 외장 디스크로 옮기세요. antmove는 원래 경로를 유지하고 이동 전에 복사본을 확인합니다.</p>
  <p>macOS 전용 · 터미널 CLI · 디스크 적합성 확인 · 이동 미리보기 · 복원 지원</p>
  <p><a href="README.md">🇺🇸 English</a> · <a href="README_JA.md">🇯🇵 日本語</a> · <a href="README_ZH_CN.md">🇨🇳 简体中文</a> · <a href="README_KO.md">🇰🇷 한국어</a> · <a href="README_FR.md">🇫🇷 Français</a> · <a href="README_UK.md">🇺🇦 Українська</a> · <a href="README_RU.md">🇷🇺 Русский</a></p>
  <p><a href="https://github.com/MakStashkevich/antmove/stargazers"><img src="https://img.shields.io/github/stars/MakStashkevich/antmove?style=flat" alt="Stars"></a> <a href="https://github.com/MakStashkevich/antmove/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a> <a href="https://github.com/MakStashkevich/antmove"><img src="https://img.shields.io/badge/version-0.1.0-green.svg" alt="Version 0.1.0"></a> <a href="https://x.com/makstashkevich"><img src="https://img.shields.io/badge/X-follow-000000?logo=x&logoColor=white" alt="Follow on X"></a> <a href="https://t.me/maksidea"><img src="https://img.shields.io/badge/Telegram-join-26A5E4?logo=telegram&logoColor=white" alt="Telegram"></a></p>
</div>

Mac 저장 공간은 꼭 앱을 설치하거나 Xcode를 업데이트하고, 게임을 받거나 iPhone을 백업하려는 순간에 모자랍니다.

나도 똑같이 겪었습니다. 앱과 앱 데이터, 개발 도구, 백업, 사진, 동영상, 음악이 내장 SSD를 꽉 채웠어요. 어느 순간엔 그냥 일하는 것까지 불편해졌습니다.

저장 공간 하나 때문에 Mac 전체를 바꾸는 건 너무 비싼 해결책처럼 느껴졌습니다. AI 회사들이 데이터센터를 계속 짓고 메모리와 저장 장치 얘기가 더 부담스러워지는 와중에, 나한테 필요한 건 새 컴퓨터가 아니라 빈 공간이었어요.

그래서 antmove를 만들었습니다. 지원되는 앱과 데이터를 외장 디스크로 옮기고 복사본을 확인한 다음, 원래 경로에 심볼릭 링크를 남깁니다. 앱은 익숙한 경로를 계속 쓰고, 실제 파일만 다른 디스크에 있는 거죠.

## 실제로는 이렇게 작동합니다

앱이 원래 주소로 찾아오면 외장 디스크 쪽으로 안내해 주는 링크를 그 자리에 남겨 두는 방식입니다.

macOS와 앱은 원래 경로를 그대로 사용합니다. 무거운 파일은 연결된 외장 디스크에 저장되고요.

필요할 때 맞는 디스크를 추가하면 됩니다. “무한 저장 공간”은 비유예요. 데이터를 쓰려면 해당 디스크를 연결해야 하고, antmove가 알아보고 확인할 수 있는 것만 옮겨야 합니다.

## antmove가 찾는 것

- <code>/Applications</code>와 <code>~/Applications</code>의 앱.
- 설치된 앱과 명확히 연결할 수 있는 대용량 데이터 폴더.
- Xcode와 Apple 개발 데이터, Android SDK/AVD, 일부 개발 도구 데이터.
- macOS가 허용하면 Photos 보관함, Music/iTunes 미디어, Apple TV 미디어/다운로드 폴더, iPhone/iPad 백업.
- 예전 Extender 이동 기록.

Docker Desktop, OrbStack, Colima, Rancher Desktop 저장 공간도 제조사 안내와 함께 표시될 수 있습니다. 하지만 antmove는 엔진 내부 저장소를 직접 옮기지 않습니다. VM과 데이터베이스를 일반 폴더처럼 움직이면 엔진이 고장 날 수 있으니까요.

큰 폴더가 어떤 앱 것인지 확실하지 않으면 antmove가 대신 찍지 않습니다. 따로 보여 주고 직접 확인하게 둡니다.

## 설치하기

Apple Silicon은 <code>arm64</code>, Intel Mac은 <code>amd64</code> 압축 파일을 받으면 됩니다. Mac 아키텍처에 맞는 파일을 [GitHub Releases](https://github.com/MakStashkevich/antmove/releases)에서 받으세요.

압축을 풀고 한 번 실행하세요.

~~~sh
cd ~/Downloads/antmove
./antmove
~~~

처음 실행하면 <code>antmove</code>와 짧은 명령 <code>am</code>을 <code>~/.local/bin</code>에 설치합니다. 이 폴더가 <code>PATH</code>에 없으면 셸 설정에 추가하니, 설치 뒤 Terminal을 다시 열어 주세요.

설치하지 않고 압축 푼 폴더에서 직접 실행해도 됩니다.

~~~sh
./antmove
~~~

명령을 직접 설치하거나 복구하려면:

~~~sh
./install-local.sh
~~~

### Bash 스크립트로 설치

<code>install.sh</code>는 macOS와 CPU 종류를 확인하고 맞는 최신 릴리스를 받아 설치합니다. 파일을 스캔하거나 옮기지는 않습니다. 평범한 Bash 스크립트라 실행 전에 링크에서 내용을 볼 수 있어요.

~~~sh
curl -fsSL https://raw.githubusercontent.com/MakStashkevich/antmove/main/install.sh | bash
~~~


### Homebrew로 설치하기

~~~sh
brew tap MakStashkevich/antmove https://github.com/MakStashkevich/antmove.git
brew install antmove
~~~

나중에 업데이트하려면:

~~~sh
brew upgrade antmove
~~~

Homebrew가 소스에서 antmove를 빌드하고 <code>antmove</code>와 <code>am</code>을 설치합니다. 빌드에 필요한 Go도 자동으로 설치됩니다.

### 직접 빌드하기

소스에서 빌드하려면 Go 1.24 이상이 필요합니다.

~~~sh
git clone https://github.com/MakStashkevich/antmove.git
cd antmove
make build
./bin/antmove install
~~~

미리 빌드된 파일을 쓰는 데 Go, Swift, Xcode는 필요하지 않습니다.

### AI 도우미에게 부탁하기

Codex, Claude Code, Cursor 같은 도구에서 쓸 antmove skill도 저장소에 있습니다. 이렇게 추가하면 됩니다.

~~~sh
npx skills add MakStashkevich/antmove -a codex claude-code cursor -g -y
~~~

Codex와 Claude Code 플러그인도 준비했습니다. 파일은 <code>.agents/plugins</code>와 <code>plugins/antmove</code>, 설치 방법은 <code>plugins/antmove/README.md</code>에 있습니다.

AI에게는 이렇게 말하면 됩니다.

> 공식 안내에 따라 antmove를 설치해 줘. 명령을 설명하고 CLI만 설치한 다음 am --version으로 확인해. 내가 말하기 전에는 파일을 스캔하거나 옮기지 마.

설치해 달라는 말이 데이터를 옮겨도 된다는 뜻은 아닙니다. 실제 작업 전에 <code>--dry-run</code> 계획을 보여 달라고 하고, 뭐가 바뀌는지 본 다음 결정하세요.

## 처음에 할 일

메뉴 실행:

~~~sh
am
~~~

방향키로 움직이고 Enter로 선택, Space로 항목을 표시합니다. 키 안내는 아래쪽에 계속 나옵니다.

터미널 화면은 영어예요. 문서는 7개 언어로 만들었습니다.

우선 뭐가 있는지 보기만 하세요.

~~~sh
am scan
~~~

그다음 쓸 만한 디스크 확인:

~~~sh
am disks
~~~

파일 시스템, 속도, 연결 방식, 남은 공간을 보면 됩니다. SSD의 APFS가 가장 권장됩니다. HFS+와 HDD의 APFS도 쓸 수 있지만 앱과 게임은 느릴 수 있어요. ExFAT, FAT, 알 수 없는 형식에는 macOS 메타데이터나 앱 호환성 문제가 생길 수 있어 경고합니다.

네트워크 디스크, 시스템 볼륨, 읽기 전용 디스크, 현재 연결되지 않은 볼륨은 대상이 될 수 없습니다.

속도 테스트는 임시 256 MiB 파일을 쓰고 읽은 다음 지웁니다.

~~~sh
am disks --test "/Volumes/Work SSD"
~~~

읽기 속도 참고값:

| 용도 | 기본 | 좋음 | 아주 좋음 |
| --- | ---: | ---: | ---: |
| 게임 | 150 MB/s | 350 MB/s | 700 MB/s |
| 사무용 앱 | 30 MB/s | 75 MB/s | 150 MB/s |
| 메신저 | 20 MB/s | 50 MB/s | 100 MB/s |

## 먼저 보고, 그다음 옮기기

나는 이렇게 할 겁니다.

1. 외장 디스크를 연결하고 <code>am disks</code>로 확인.
2. <code>am scan</code>에서 이름, 크기, 상태, 메모 확인.
3. 관련 앱을 종료하고 백업이나 동기화가 끝날 때까지 기다리기.
4. 메뉴에서 **Move data**를 골라 항목과 디스크를 지정하고, 경고를 읽은 다음 확인.
5. 옮긴 데이터를 쓰는 동안 디스크 연결해 두기.

antmove가 앱이나 컨테이너 엔진을 대신 종료하지는 않습니다. 앱이 열려 있으면 직접 닫고 다시 스캔하세요.

<code>am scan</code>에서 나온 ID로 먼저 미리보기:

~~~sh
am move --disk "/Volumes/Work SSD" --dry-run 71a29f5a1cd0
~~~

이 명령은 복사하거나 바꾸지 않습니다. 계획을 확인하고 괜찮으면 실제로 옮기세요.

~~~sh
am move --disk "/Volumes/Work SSD" --yes 71a29f5a1cd0
~~~

권장 디스크로 비대화형 이동할 때는 <code>--yes</code>를 씁니다. 주의가 표시된 디스크는 두 번 따로 확인해야 합니다.

~~~sh
am move --disk "/Volumes/Shared" --confirm-caution --confirm-caution 71a29f5a1cd0
~~~

확인부터 누르지 말고 왜 경고가 떴는지 보세요. 디스크가 느리면 게임도 답답해지고, 파일 시스템에 따라 macOS 메타데이터가 보존되지 않을 수 있습니다. 두 번 확인한다고 추천 디스크가 되는 건 아닙니다.

antmove는 여유 공간을 확인하고 macOS <code>ditto</code>로 임시 폴더에 복사한 뒤 해시와 디렉터리 내용을 비교합니다. 다 확인하고 나서 원래 경로에 심볼릭 링크를 둡니다. 작업 기록은 <code>~/Library/Application Support/antmove</code>에 저장됩니다.

## 사진, 음악, 영화, 백업

- **Photos:** <code>.photoslibrary</code>를 옮기기 전에 Photos를 종료하세요. Apple은 APFS 또는 Mac OS Extended를 권장합니다. 이동 뒤 새 위치에서 보관함을 열고, iCloud Photos를 쓴다면 필요할 때 시스템 사진 보관함으로 지정하세요.
- **Music:** 먼저 Music을 닫으세요. 이후 파일을 가져올 거라면 **Music → Settings → Files → Media Location**을 확인하세요. antmove는 설정을 바꾸지 않습니다.
- **Apple TV:** TV를 닫으세요. 다운로드 영화에 DRM이 걸려 있을 수 있습니다. 옮긴 다음 재생이 되는지 보고, 볼 때 디스크를 연결해 두세요.
- **iPhone/iPad 백업:** 기본 경로는 <code>~/Library/Application Support/MobileSync/Backup</code>입니다. Finder 백업이나 동기화가 끝난 뒤 옮기세요. 접근 거부가 뜨면 <code>am</code>을 실행하는 터미널 앱에 Full Disk Access를 허용하고 다시 연 다음 재검사하세요.
- **Docker:** 엔진 자체의 이전 방법을 쓰세요. VM이나 데이터베이스 파일을 일반 폴더처럼 옮기는 건 권하지 않습니다.

Apple 안내: [Photos 보관함 옮기기](https://support.apple.com/en-ie/108345) · [Music 파일 설정](https://support.apple.com/en-ca/guide/music/mus69248042d/mac) · [Apple TV 설정](https://support.apple.com/en-gb/guide/tvapp-mac/tv0f5accea/mac) · [iPhone 백업 찾기](https://support.apple.com/en-la/108809).

## 원래대로 돌리고 싶을 때

먼저 복원 계획 보기:

~~~sh
am restore --dry-run 71a29f5a1cd0
~~~

괜찮으면 복원:

~~~sh
am restore --yes 71a29f5a1cd0
~~~

<code>am status</code>에서 기록과 디스크 연결 상태를 볼 수 있습니다. 작업이 중단되면 디스크를 연결하고 <code>am recover</code>를 실행하세요.

antmove 제거에는 확인이 두 번 필요합니다.

~~~sh
am uninstall
~~~

제거해도 옮긴 데이터는 돌아오지 않습니다. 먼저 <code>am restore</code>를 쓰거나 디스크와 링크를 그대로 두세요.

## 명령 모음

| 명령 | 하는 일 |
| --- | --- |
| <code>am</code> | 메뉴 열기 |
| <code>am install</code> | 명령 설치/복구 |
| <code>am uninstall</code> | 두 번 확인 후 제거 |
| <code>am scan</code> | 후보와 상태 찾기 |
| <code>am disks</code> | 쓸 수 있는 디스크 표시 |
| <code>am disks --test PATH</code> | 읽기/쓰기 속도 측정 |
| <code>am move --disk PATH [ID…]</code> | 선택 항목 이동 |
| <code>am restore [ID…]</code> | 원래 경로로 복원 |
| <code>am status</code> | 기록과 디스크 상태 |
| <code>am recover</code> | 중단된 작업 복구 |
| <code>am migrate --dry-run</code> | Extender 기록 가져오기 미리보기 |
| <code>am migrate</code> | Extender 기록 가져오기 |
| <code>am --help</code> | 도움말 |
| <code>am --version</code> | 버전과 시스템 정보 |

## 직접 빌드하고 싶다면

Go 1.24 이상이 필요합니다. macOS에서 현재 컴퓨터용 또는 두 아키텍처용 빌드가 가능합니다.

~~~sh
make build
make build-arm64
make build-amd64
make release
make test
make check
~~~

압축 파일은 <code>dist/</code>에 만들어집니다. 현재 바이너리는 서명/공증되지 않아 Gatekeeper가 **System Settings → Privacy & Security**에서 실행을 허용해 달라고 할 수 있습니다.

## 라이선스

antmove는 [MIT License](LICENSE)로 배포됩니다.

