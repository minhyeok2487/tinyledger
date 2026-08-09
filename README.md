# tinyledger (가계부)

혼자 쓰는 가벼운 가계부 웹앱. Go 단일 바이너리 + SQLite(로컬) 또는 Turso(클라우드), 프론트엔드 프레임워크 없이 서버사이드 렌더링만으로 동작합니다.

## 주요 기능

- 대시보드: 이번 달 수입/지출/실제 잔액, 카테고리별 지출 바 그래프
- 캘린더: 날짜별 수입/지출 한눈에 보기
- 검색: 키워드/카테고리/기간/계좌로 내역 필터링
- 즐겨찾기: 자주 쓰는 항목을 칩으로 저장해서 원클릭 입력, 이번 달에 이미 쓴 항목은 자동으로 목록에서 제외
- 다중 계좌: 계좌별 실제 잔액 관리, 계좌간 이체
- 메모: 홈 화면에 접었다 펼치는 메모 카드 (수익관리 방침 등 기록용)

## 스택

- Go (`net/http`, `html/template`, 표준 라이브러리 위주)
- DB: [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (순수 Go, CGO 불필요) 또는 [Turso](https://turso.tech) (libSQL 클라우드)
- 프론트: 서버사이드 렌더링 + 약간의 vanilla JS, 빌드 스텝 없음

## 로컬 실행

```bash
go run .
```

기본적으로 `http://localhost:8080` 에서 뜨고, 실행 위치에 `gagyebu.db` (SQLite 파일)가 자동 생성됩니다.

포트나 DB 경로를 바꾸려면:

```bash
PORT=3000 GAGYEBU_DB=/path/to/data.db go run .
```

## 클라우드 DB로 전환하기 (Turso)

로컬 파일 대신 Turso(클라우드 SQLite)를 쓰면 서버를 내렸다 올려도, 어디서 배포하든 데이터가 유지됩니다.

1. Turso CLI 설치 (macOS/Linux는 공식 설치 스크립트, Windows는 Go로 빌드):

   ```bash
   # macOS/Linux
   curl -sSfL https://get.tur.so/install.sh | bash

   # Windows (Go 설치되어 있으면)
   go install github.com/tursodatabase/turso-cli/cmd/turso@latest
   ```

2. 로그인 및 DB 생성:

   ```bash
   turso auth login
   turso db create tinyledger
   turso db show tinyledger        # URL 확인
   turso db tokens create tinyledger   # 인증 토큰 발급
   ```

3. 기존 로컬 데이터를 그대로 옮기고 싶다면:

   ```bash
   python -c "
   import sqlite3
   conn = sqlite3.connect('gagyebu.db')
   with open('dump.sql', 'w', encoding='utf-8') as f:
       f.write('PRAGMA foreign_keys=OFF;\n')
       for line in conn.iterdump():
           if 'sqlite_sequence' not in line:
               f.write(line + '\n')
   "
   turso db shell tinyledger < dump.sql
   ```

4. 프로젝트 루트에 `.env` 파일 생성:

   ```env
   TURSO_DATABASE_URL=libsql://<db-name>-<org>.turso.io
   TURSO_AUTH_TOKEN=<발급받은 토큰>
   ```

   `.env`가 있으면 앱이 자동으로 Turso를 쓰고, 없으면 로컬 SQLite로 동작합니다 (`main.go` / `db.go` 참고).

## 배포

### 방법 A — Fly.io / Railway / VPS (Docker, 추천)

Go의 `net/http` 상시 서버 구조라 이 방식이 가장 자연스럽습니다. 저장소에 포함된 `Dockerfile`을 그대로 사용하세요.

```bash
# Fly.io 예시
fly launch --no-deploy
fly secrets set TURSO_DATABASE_URL=... TURSO_AUTH_TOKEN=...
fly deploy
```

Railway / 일반 VPS도 동일하게 Docker 이미지를 빌드해서 `TURSO_DATABASE_URL`, `TURSO_AUTH_TOKEN`, `PORT` 환경변수만 주입하면 됩니다.

### 방법 B — Vercel

Vercel은 서버리스 구조라 지금의 `net/http.ListenAndServe` 상시 서버 형태가 그대로 올라가지 않습니다. 올리려면:

1. DB를 Turso(또는 다른 원격 DB)로 전환 (위 가이드 참고) — 로컬 파일 DB는 서버리스 환경에서 유지되지 않습니다.
2. `main.go`의 라우팅을 Vercel Go 런타임 규격(`api/index.go`에서 `Handler(w, r)` export)에 맞게 감싸는 작업이 필요합니다.
3. `vercel.json`에 모든 경로를 하나의 함수로 라우팅하는 rewrite 설정 추가.

현재 저장소는 방법 A(상시 서버) 기준으로 구성되어 있습니다.

## 환경변수 요약

| 변수 | 설명 | 기본값 |
|---|---|---|
| `PORT` | 서버 포트 | `8080` |
| `GAGYEBU_DB` | 로컬 SQLite 파일 경로 (Turso 미설정 시) | `gagyebu.db` |
| `TURSO_DATABASE_URL` | Turso DB URL (설정 시 로컬 DB 대신 사용) | - |
| `TURSO_AUTH_TOKEN` | Turso 인증 토큰 | - |
| `GAGYEBU_PASSWORD` | 접속 비밀번호 (배포 시 필수) | - |

## 비밀번호 잠금

`GAGYEBU_PASSWORD`를 설정하면 모든 화면이 로그인 뒤로 숨겨집니다. 로그인하면
서명된 세션 쿠키가 30일간 유지되고, 계좌 화면 하단의 로그아웃 버튼으로 해제할
수 있습니다. 비밀번호를 바꾸면 기존 세션은 모두 무효가 됩니다.

로컬 개발에서는 값을 비워두면 잠금 없이 동작합니다. 다만 **Vercel에서는 값이
없으면 503으로 응답을 거부**합니다 — 환경변수를 깜빡한 배포가 가계부를 공개
상태로 노출하지 않도록 하기 위한 동작입니다.

```bash
GAGYEBU_PASSWORD='원하는-비밀번호' PORT=3000 go run .
```

Vercel에서는 프로젝트 설정 → Environment Variables에 `GAGYEBU_PASSWORD`를
추가한 뒤 재배포하면 됩니다.

## 라이선스

개인 프로젝트, 별도 라이선스 없이 자유롭게 참고/포크하세요.
