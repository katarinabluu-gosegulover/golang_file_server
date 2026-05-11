# Go Secure File Share

Golang으로 구현한 파일 업로드/공유 백엔드 예제입니다. 웹 UI와 `curl` 양쪽에서 업로드할 수 있고, 업로드된 파일은 서버 내부 안전한 디렉터리에 저장된 뒤 ID 기반 공유 링크로만 다운로드됩니다.

## 실행

```powershell
go run ./cmd/server
```

브라우저에서 `http://localhost:8080`을 엽니다.

환경 변수:

```powershell
$env:ADDR=":8080"
$env:FILE_SHARE_DATA_DIR="data"
$env:MAX_UPLOAD_BYTES="10485760"
$env:BASE_URL="https://your-domain.example"
go run ./cmd/server
```

`BASE_URL`은 외부 서버에 배포했을 때 공유 링크에 들어갈 공개 도메인입니다.
실제로 배포한 웹 주소는 https://golang-file-server.onrender.com 입니다.

## curl 사용

업로드:

```powershell
curl.exe -F "file=@sample.png" http://localhost:8080/upload
```

응답 예시:

```json
{
  "id": "8a8a4f4a6d0d4a6f80caa0a88e72a6dd",
  "original_name": "sample.png",
  "content_type": "image/png",
  "size": 1204,
  "sha256": "...",
  "share_url": "http://localhost:8080/share/8a8a4f4a6d0d4a6f80caa0a88e72a6dd",
  "download_url": "http://localhost:8080/share/8a8a4f4a6d0d4a6f80caa0a88e72a6dd"
}
```

다운로드:

```powershell
curl.exe -L "http://localhost:8080/share/8a8a4f4a6d0d4a6f80caa0a88e72a6dd" -o downloaded.png
```

삭제:

```powershell
curl.exe -X DELETE "http://localhost:8080/share/8a8a4f4a6d0d4a6f80caa0a88e72a6dd"
```

## 보안 처리

- 업로드 파일은 `data/uploads` 아래에 저장하며, 원본 파일명은 저장 경로에 사용하지 않습니다.
- 저장 파일명은 `crypto/rand`로 만든 128비트 ID와 검증된 확장자로 구성합니다.
- 공유 링크는 `/share/{id}` 형식이고, ID는 32자리 hex 문자열만 허용합니다.
- 허용 확장자와 감지된 MIME 타입을 함께 검사합니다.
- `.json` 파일은 MIME 검사 후 실제 JSON 문법도 확인합니다.
- 파일 크기는 기본 10 MiB로 제한합니다.
- 다운로드 응답은 `Content-Disposition: attachment`와 `X-Content-Type-Options: nosniff`를 사용합니다.
- 기본 응답에 `Content-Security-Policy`, `X-Frame-Options`, `Referrer-Policy` 보안 헤더를 적용합니다.
- 경로 이탈을 막기 위해 최종 저장 경로가 업로드 디렉터리 안인지 검증합니다.

허용 확장자:

`.txt`, `.csv`, `.json`, `.pdf`, `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.zip`, `.md`

## 테스트

```powershell
go test ./...
```

테스트는 정상 업로드/다운로드, 경로 조작 파일명, 허용되지 않은 확장자, MIME 불일치, 잘못된 JSON, 잘못된 공유 ID를 검증합니다.

## 배포 체크리스트

Render로 배포하는 구체적인 순서는 [DEPLOY_RENDER.md](./DEPLOY_RENDER.md)에 정리했습니다.

1. 서버에서 Go를 설치하거나 바이너리를 빌드해서 업로드합니다.
2. 리버스 프록시(Nginx, Caddy 등)에서 HTTPS를 적용합니다.
3. `BASE_URL=https://your-domain.example`를 지정합니다.
4. `FILE_SHARE_DATA_DIR`를 영속 디스크 경로로 지정합니다.
5. 업로드 용량이 필요 이상으로 크지 않게 `MAX_UPLOAD_BYTES`를 조정합니다.
6. `/healthz`로 상태 확인을 붙입니다.

Linux 빌드 예시:

```powershell
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o fileshare ./cmd/server
```
