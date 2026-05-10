# Golang 파일 업로드/공유 서버 구현 정리

## 목표

Go로 웹 백엔드 구조를 익히기 위해 파일 업로드와 외부 공유 링크 생성 기능을 직접 구현했다. 웹 브라우저뿐 아니라 `curl` 명령어로도 업로드와 다운로드가 가능하도록 만들고, 업로드 과정에서 MIME 타입, 확장자, 경로 검증을 적용했다.

## 구현 구조

```text
go-secure-file-share/
  cmd/server/main.go
  internal/fileshare/server.go
  internal/fileshare/storage.go
  internal/fileshare/validation.go
  internal/fileshare/server_test.go
```

`cmd/server/main.go`는 환경 변수를 읽고 HTTP 서버를 실행한다. 실제 업로드 처리, 파일 저장, 검증 로직은 `internal/fileshare` 패키지로 분리했다. 이렇게 나누면 서버 실행 코드와 비즈니스 로직을 따로 테스트할 수 있다.

## 파일 업로드 흐름

1. 사용자가 웹 UI 또는 `curl -F "file=@sample.png"`로 `/upload`에 multipart 요청을 보낸다.
2. 서버는 요청 본문 크기를 제한하고 multipart form을 파싱한다.
3. 원본 파일명에서 경로 문자가 있는지 확인하고 허용 확장자인지 검사한다.
4. 파일 앞부분 512바이트를 읽어 `http.DetectContentType`으로 MIME 타입을 감지한다.
5. 확장자와 MIME 타입이 허용된 조합인지 확인한다.
6. `.json` 파일은 실제 JSON 문법까지 확인한다.
7. 랜덤 ID를 생성하고 `data/uploads/{id}{ext}` 형태로 저장한다.
8. 메타데이터를 `data/metadata.json`에 기록하고 `/share/{id}` 공유 링크를 반환한다.

핵심 검증 코드는 다음과 같다.

```go
contentType := detectMediaType(head[:n])
if err := validateDetectedType(ext, contentType); err != nil {
    return UploadResponse{}, err
}
```

파일명은 저장 경로로 직접 쓰지 않았다. 원본 파일명에는 `../secret.txt` 같은 경로 조작 문자열이 들어갈 수 있기 때문이다. 실제 저장 파일명은 서버가 만든 랜덤 ID만 사용한다.

## 공유 링크와 다운로드

업로드 성공 시 서버는 `/share/{id}` 형식의 공유 링크를 만든다. 다운로드 요청에서는 ID가 32자리 hex 문자열인지 먼저 확인한다. 그 다음 메타데이터에서 실제 저장 파일명을 찾고, 최종 경로가 업로드 디렉터리 안에 있는지 다시 검증한다.

다운로드 응답에는 다음 보안 헤더를 적용했다.

```go
w.Header().Set("Content-Disposition", contentDispositionAttachment(meta.OriginalName))
w.Header().Set("X-Content-Type-Options", "nosniff")
```

브라우저가 파일을 페이지 안에서 실행하거나 MIME 타입을 임의로 추측하지 않도록 하기 위해서다.

## curl 테스트

업로드:

```powershell
curl.exe -F "file=@sample.png" http://localhost:8080/upload
```

다운로드:

```powershell
curl.exe -L "http://localhost:8080/share/{id}" -o downloaded.png
```

웹 UI는 같은 `/upload` 엔드포인트를 사용한다. 즉, 브라우저와 CLI가 같은 백엔드 로직을 공유한다.

## 테스트한 보안 케이스

- 정상 PNG 업로드 후 공유 링크로 다운로드
- `../pixel.png` 같은 경로 조작 파일명 거부
- `.exe` 같은 허용되지 않은 확장자 거부
- 내용은 텍스트인데 확장자만 `.png`인 MIME 불일치 파일 거부
- 확장자는 `.json`이지만 JSON 문법이 깨진 파일 거부
- `/share/../../secret` 같은 잘못된 공유 ID 거부

테스트 실행:

```powershell
go test ./...
```

## 배운 점

파일 업로드 기능은 단순히 파일을 받아 저장하는 기능처럼 보이지만, 실제로는 입력값을 신뢰하지 않는 설계가 중요하다. 원본 파일명, 확장자, MIME 타입, 저장 경로, 다운로드 방식 모두 공격 표면이 될 수 있다. 특히 원본 파일명을 경로로 사용하지 않고, 서버가 생성한 ID로만 저장하는 방식이 경로 조작 위험을 줄이는 데 효과적이었다.

또한 웹 UI와 `curl` API를 같은 핸들러로 처리하면 기능 중복이 줄어든다. 브라우저 요청에는 리다이렉트로 응답하고, `curl` 요청에는 JSON으로 응답하도록 `Accept` 헤더를 기준으로 나누었다.

## 배포 시 확인할 점

실제 외부 접근 가능한 서버에 배포할 때는 HTTPS가 필수다. Nginx나 Caddy 같은 리버스 프록시를 앞에 두고 TLS 인증서를 적용한다. 공유 링크 생성을 위해 `BASE_URL=https://도메인` 환경 변수를 지정하고, 업로드 파일이 사라지지 않도록 `FILE_SHARE_DATA_DIR`를 영속 디스크로 지정한다.
