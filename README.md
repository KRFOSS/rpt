# rpt — ROKFOSS 패키지 관리자

apt 와 dpkg 가 없는 시놀로지 DSM 에서 [pkg.krfoss.org](https://pkg.krfoss.org) 의
데비안 패키지를 설치하고 관리하는 도구입니다. deb 파일을 하나씩 내려받아
손으로 푸는 대신 `rpt install krfs-rport` 한 줄로 끝납니다.

```
rpt update
rpt install krfs-rport -y
rpt list --installed
rpt remove krfs-rport
```

## 무엇이 다른가

시스템 패키지 관리자와 **절대 섞이지 않습니다.** `/var/lib/dpkg` 를 읽지도
쓰지도 않고, 설치 목록은 rpt 자신의 루트 아래에만 둡니다. rpt 가 만든 것만
rpt 가 지우므로 실수로 시스템 파일을 건드릴 일이 없습니다.

- **DSM 업데이트에 살아남습니다.** 시놀로지는 업데이트할 때 시스템 파티션을
  다시 씌우기 때문에 `/usr/bin` 에 설치하면 전부 날아갑니다. rpt 는 볼륨 안
  `/volume1/@rokfoss` 에 풀고 `/usr/local/bin` 에는 심링크만 겁니다. 업데이트
  뒤에 링크가 사라졌다면 `rpt relink` 한 번이면 됩니다.
- **서명을 검증합니다.** 저장소 GPG 키를 바이너리에 내장해 InRelease 서명을
  확인하고, 거기 적힌 SHA256 으로 패키지 목록을, 목록에 적힌 SHA256 으로
  deb 파일을 확인합니다.
- **의존성 없는 단일 바이너리입니다.** ar 과 tar, zstd 처리를 모두 Go 로
  구현해 외부 명령을 부르지 않습니다. 파일 하나만 올리면 동작합니다.
- **아키텍처가 맞는 패키지만 다룹니다.** x86_64 기기에서는 amd64 목록만
  내려받으므로 ARM 전용 패키지는 애초에 목록에 오르지 않습니다.

## 설치

Go 1.25 이상에서 정적 바이너리를 만들어 NAS 에 올립니다.

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o rpt .
scp rpt root@nas:/usr/local/bin/rpt
```

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

`-y` 는 확인 질문을 건너뜁니다. 명령과 옵션 이름은 apt 와 같게 맞췄습니다.

## 환경 변수

| 변수 | 기본값 | 설명 |
| --- | --- | --- |
| `RPT_ROOT` | 첫 볼륨의 `@rokfoss` | 패키지를 푸는 설치 루트 |
| `RPT_REPO` | `https://pkg.krfoss.org/debian` | 저장소 주소 |
| `RPT_BINDIR` | `/usr/local/bin` | 실행 파일 심링크 위치 |
| `RPT_ETCDIR` | `/etc` | 설정 디렉터리 심링크 위치 |

볼륨이 없는 환경에서는 설치 루트가 `/opt/rokfoss` 로 잡힙니다.

## 배치 구조

```
/volume1/@rokfoss/
├── usr/bin/krfs-rport          실제 실행 파일
├── etc/krfs-rport/config.conf  설정 파일
├── var/lib/rpt/status.json     설치 목록 (rpt 전용)
├── var/lib/rpt/lists/          저장소 목록 캐시
└── var/cache/rpt/archives/     내려받은 deb

/usr/local/bin/krfs-rport  ->  /volume1/@rokfoss/usr/bin/krfs-rport
/etc/krfs-rport            ->  /volume1/@rokfoss/etc/krfs-rport
```

심링크를 걸 자리에 이미 다른 파일이 있으면 그대로 두고 경고만 합니다.
제거할 때도 rpt 가 만든 심링크인지 확인한 뒤에만 걷어냅니다.

## 알아 둘 점

- **관리자 스크립트는 실행하지 않습니다.** 패키지의 `postinst` 는 보통
  `systemctl enable` 을 부르는데, DSM 의 `synopkg` 체계와 별개로 유닛을 꽂는
  것은 위험합니다. 설치 후 어떤 스크립트를 건너뛰었는지 알려 주므로 필요하면
  직접 처리하십시오.
- **저장소 밖 의존성은 경고만 합니다.** `libc6`, `rsync`, `openssh-client`
  같은 것은 DSM 에 이미 들어 있어 rpt 가 따로 설치하지 않습니다.
- **설정 파일은 덮어쓰지 않습니다.** 재설치나 업그레이드에서 사용자가 고친
  `conffiles` 는 그대로 둡니다. 지우려면 `purge` 를 쓰십시오.

## 릴리스

`v` 로 시작하는 태그를 밀면 GitHub Actions 가 amd64 와 arm64 용 정적
바이너리를 빌드해 deb, rpm, pacman 패키지를 만들고 체크섬과 함께 릴리스에
올립니다.

```sh
git tag v0.1.0 && git push origin v0.1.0
```

## 라이선스

MIT. 자세한 내용은 [LICENSE](LICENSE) 를 보십시오.
