# rpt — ROKFOSS 패키지 관리자

apt 와 dpkg 가 없는 Linux 환경에서 [pkg.krfoss.org](https://pkg.krfoss.org) 의
데비안 패키지를 설치하고 관리하는 도구입니다. deb 파일을 하나씩 내려받아
손으로 푸는 대신 `rpt install krfs-rport` 한 줄로 끝납니다.

## 현재까지 사용 가능이 확인된 운영체제
- Debian 계열
- Arch 계열
- Synology

```
rpt update
rpt install krfs-rport -y
rpt list --installed
rpt remove krfs-rport
```

## 무엇이 다른가

시스템 패키지 관리자와 **절대 섞이지 않습니다.** `/var/lib/dpkg` 를 읽지도
쓰지도 않고, 설치 목록과 캐시는 rpt 가 정한 위치에만 둡니다. rpt 가 만든 것만
rpt 가 지우므로 실수로 시스템 파일을 건드릴 일이 없습니다.

- **시스템 파일과 분리됩니다.** 설치 루트와 상태/캐시 경로를 분리해 두므로
  배포판의 패키지 관리자와 충돌하지 않습니다. 필요하면 `RPT_ROOT`,
  `RPT_STATEDIR`, `RPT_CACHEDIR` 로 배치를 조정할 수 있습니다.
- **서명을 검증합니다.** 저장소 GPG 키를 바이너리에 내장해 InRelease 서명을
  확인하고, 거기 적힌 SHA256 으로 패키지 목록을, 목록에 적힌 SHA256 으로
  deb 파일을 확인합니다.
- **의존성 없는 단일 바이너리입니다.** ar 과 tar, zstd 처리를 모두 Go 로
  구현해 외부 명령을 부르지 않습니다. 파일 하나만 올리면 동작합니다.
- **아키텍처가 맞는 패키지만 다룹니다.** x86_64 기기에서는 amd64 목록만
  내려받으므로 ARM 전용 패키지는 애초에 목록에 오르지 않습니다.

## 설치

[릴리스](https://github.com/KRFOSS/rpt/releases) 에서 배포판에 맞는 파일을 받아
설치합니다. amd64 와 arm64 를 모두 제공합니다.

```sh
sudo dpkg -i rpt_1.0.2-1_amd64.deb                 # Debian 계열
sudo rpm -i rpt-1.0.2-1.x86_64.rpm                 # RPM 계열
sudo pacman -U rpt-1.0.2-1-x86_64.pkg.tar.zst      # Arch 계열
```

소스에서 직접 빌드하려면 Go 1.25 이상이 필요합니다. 외부 라이브러리를
링크하지 않으므로 결과물은 어느 배포판에서나 그대로 도는 정적 바이너리입니다.

```sh
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o rpt .
```

이렇게 빌드하면 `rpt version` 이 `dev` 라고 답합니다. 버전은 릴리스 빌드에서
링커가 넣습니다. 직접 넣으려면 `-X` 를 더하십시오.

```sh
CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X github.com/krfoss/rpt/internal/config.Version=1.2.3" -o rpt .
```

시놀로지는 dpkg 가 없어 절차가 다릅니다. [시놀로지 DSM](#시놀로지-dsm) 을 보십시오.

## 명령

| 명령 | 설명 |
| --- | --- |
| `rpt update` | 저장소 패키지 목록을 갱신합니다 |
| `rpt install <패키지...>` | 패키지와 의존성을 설치합니다 |
| `rpt remove <패키지...>` | 패키지를 지웁니다 (설정 파일은 남깁니다) |
| `rpt purge <패키지...>` | 설정 파일까지 함께 지웁니다 |
| `rpt upgrade [패키지...]` | 최신 버전으로 올립니다 |
| `rpt autoremove` | 딸려왔다가 필요 없어진 패키지를 지웁니다 |
| `rpt list` | 패키지 목록을 봅니다 (`--installed`, `--upgradable`) |
| `rpt search <검색어>` | 이름과 설명에서 찾습니다 |
| `rpt show <패키지>` | 상세 정보를 봅니다 |
| `rpt clean` | 내려받아 둔 deb 를 모두 지웁니다 |
| `rpt autoclean` | 저장소에서 사라진 옛 deb 만 지웁니다 |
| `rpt relink` | 시스템 심링크를 다시 만듭니다 |
| `rpt web [stop\|restart]` | 로컬 웹 대시보드를 열거나 멈춥니다 |
| `rpt completion <셸>` | bash, zsh 자동완성 스크립트를 출력합니다 |
| `rpt web` | 로컬 웹 대시보드를 엽니다 (`127.0.0.1:47981`) |

`-y` 는 확인 질문을 건너뜁니다. 명령과 옵션 이름은 apt 와 같게 맞췄고,
`rpt update` 의 출력도 apt 와 같은 형식입니다.

## 자동완성

deb, rpm, pacman 패키지로 설치하면 bash 와 zsh 자동완성이 함께 깔립니다.
새 셸을 열면 명령과 옵션은 물론 패키지 이름까지 탭으로 완성됩니다.
`install` 은 저장소의 패키지를, `remove` 는 설치된 패키지를 채웁니다.
이름은 그때그때 `rpt list` 를 불러 얻으므로 저장소가 바뀌어도 다시 깔
필요가 없습니다.

소스에서 빌드했다면 직접 넣으십시오.

```sh
rpt completion bash | sudo tee /usr/share/bash-completion/completions/rpt >/dev/null
rpt completion zsh  | sudo tee /usr/share/zsh/site-functions/_rpt >/dev/null
```

## 환경 변수

| 변수 | 기본값 | 설명 |
| --- | --- | --- |
| `RPT_ROOT` | `/opt/rpt` | 패키지를 푸는 설치 루트 |
| `RPT_STATEDIR` | `/var/lib/rpt` | 상태 파일과 목록 캐시 위치 |
| `RPT_CACHEDIR` | `/var/cache/rpt` | 내려받은 deb 캐시 위치 |
| `RPT_REPO` | `https://pkg.krfoss.org/debian` | 저장소 주소 |
| `RPT_BINDIR` | `/usr/local/bin` | 실행 파일 심링크 위치 |
| `RPT_ETCDIR` | `/etc` | 설정 디렉터리 심링크 위치 |
| `RPT_WEB_ADDR` | `127.0.0.1:47981` | 로컬 웹 대시보드 주소 |

시놀로지 환경에서는 `/volumeN/@rokfoss` 를 우선 사용하고, 일반 Linux에서는
기본적으로 `/opt/rpt` 와 `/var/lib/rpt`, `/var/cache/rpt` 를 사용합니다.

`RPT_ROOT` 를 직접 지정하면 상태와 캐시도 그 아래로 따라갑니다
(`<루트>/var/lib/rpt`, `<루트>/var/cache/rpt`). 설치물을 한 곳에 모아 두기
위한 동작이며, 따로 흩어 두려면 `RPT_STATEDIR` 과 `RPT_CACHEDIR` 을 함께
지정하십시오.

## 배치 구조

```
/opt/rpt/
├── usr/bin/krfs-rport          실제 실행 파일
└── etc/krfs-rport/config.conf  설정 파일

/var/lib/rpt/status.json        설치 목록 (rpt 전용)
/var/lib/rpt/lists/             저장소 목록 캐시
/var/cache/rpt/archives/        내려받은 deb

/usr/local/bin/krfs-rport  ->  /opt/rpt/usr/bin/krfs-rport
/etc/krfs-rport            ->  /opt/rpt/etc/krfs-rport
```

심링크를 걸 자리에 이미 다른 파일이 있으면 그대로 두고 경고만 합니다.
제거할 때도 rpt 가 만든 심링크인지 확인한 뒤에만 걷어냅니다.

## 웹 지원

`rpt web` 를 실행하면 `127.0.0.1:47981` 에서 웹 대시보드를 엽니다.
브라우저에서 `update`, `install`, `remove`, `purge`, `upgrade`, `autoremove`, `list`, `search`, `show`, `clean`, `autoclean`, `relink`, `version`, `help` 를 모두 실행할 수 있습니다.
상태 경로, 캐시 경로, 저장소 메타데이터, 설치된 패키지 목록도 함께 볼 수 있습니다.

```sh
rpt web                      # 백그라운드로 띄웁니다
rpt web restart              # 멈췄다 다시 띄웁니다 (새로 빌드한 뒤에 씁니다)
rpt web stop                 # 멈춥니다
rpt web --addr 0.0.0.0:8080  # 주소를 바꿔 띄웁니다
```

서버는 백그라운드로 돌아가며, 어느 프로세스인지는 상태 디렉터리의
`web.pid` 에 적어 둡니다. `stop` 은 처리 중인 요청이 끝나기를 기다렸다가
멈추므로 설치가 한창일 때 멈춰도 설치 기록이 날아가지 않습니다.

## 알아 둘 점

- **관리자 스크립트는 실행하지 않습니다.** 패키지의 `postinst` 는 보통
  `systemctl enable` 이나 사용자 추가 같은 일을 하는데, 시스템 패키지 관리자
  바깥에서 그런 변경을 가하는 것은 위험합니다. 시놀로지처럼 `synopkg` 가
  서비스를 관리하는 환경이라면 더욱 그렇습니다. 설치 후 어떤 스크립트를
  건너뛰었는지 알려 주므로 필요하면 직접 처리하십시오.
- **저장소 밖 의존성은 경고만 합니다.** `libc6`, `rsync`, `openssh-client`
  같은 것은 대개 시스템에 이미 있으므로 rpt 가 따로 설치하지 않고 이름만
  알려 줍니다. 없다면 배포판의 패키지 관리자로 설치하십시오.
- **설정 파일은 덮어쓰지 않습니다.** 재설치나 업그레이드에서 사용자가 고친
  `conffiles` 는 그대로 둡니다. 지우려면 `purge` 를 쓰십시오.

## 시놀로지 DSM

DSM 은 업데이트할 때 시스템 파티션을 다시 씌웁니다. `/usr/bin` 이나 `/bin` 에
둔 파일은 그때 사라지므로, 시놀로지에서는 설치 루트를 볼륨 안
`/volumeN/@rokfoss` 로 잡습니다. 시스템 경로에는 심링크만 걸고, 업데이트로
링크가 사라지면 `rpt relink` 한 번으로 되살립니다.

dpkg 가 없어 rpt 자신은 처음 한 번만 손으로 올려야 합니다. dpkg 가 있는
리눅스에서 바이너리를 꺼내 옮긴 뒤, 그것으로 rpt 를 다시 설치하면 됩니다.

```sh
# dpkg 가 있는 리눅스에서
dpkg-deb -x rpt_1.0.2-1_amd64.deb out
scp out/usr/bin/rpt root@nas:/tmp/rpt

# 시놀로지에서
chmod +x /tmp/rpt
/tmp/rpt update
/tmp/rpt install rpt -y
rm /tmp/rpt
```

이후로는 `rpt update && rpt upgrade -y` 로 rpt 자신까지 함께 갱신됩니다.
실행 중인 자기 바이너리도 교체할 수 있으므로 다시 손으로 풀 일은 없습니다.

부트스트랩 사본을 `/usr/local/bin` 이나 `/usr/bin` 에 두지 마십시오. rpt 는
자기가 만들지 않은 파일을 덮어쓰지 않으므로 심링크를 걸지 못하고, 사본이
갈려 업그레이드해도 옛 것이 계속 실행됩니다. `/tmp` 에 두고 설치 후 지우면
이 문제가 없습니다.

설치한 명령이 곧바로 안 잡히면 셸이 옛 경로를 캐시한 것입니다. DSM 의 ash 는
명령 경로를 기억하므로, 예전에 같은 이름의 파일이 다른 자리에 있었다면
`No such file or directory` 가 계속 납니다. `hash -r` 을 실행하거나 다시
로그인하십시오.

## 릴리스

`v` 로 시작하는 태그를 밀면 GitHub Actions 가 amd64 와 arm64 용 정적
바이너리를 빌드해 deb, rpm, pacman 패키지를 만들고 체크섬과 함께 릴리스에
올립니다.

```sh
git tag v1.0.3 && git push origin v1.0.3
```

## 라이선스

MIT. 자세한 내용은 [LICENSE](LICENSE) 를 보십시오.
