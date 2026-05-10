# Render 배포 순서

이 프로젝트는 Go 백엔드 서버이므로 GitHub Pages가 아니라 Render의 Web Service 같은 서버 실행 환경에 배포해야 합니다.

## 1. GitHub 저장소 만들기

`go-secure-file-share` 폴더만 별도 저장소로 올리는 것을 추천합니다. 현재 상위 폴더에는 다른 실습 파일이 많기 때문입니다.

```powershell
cd "C:\Users\jinse\OneDrive\문서\New project\go-secure-file-share"
git init
git add .
git commit -m "Initial secure file share server"
git branch -M main
git remote add origin https://github.com/<본인아이디>/<저장소이름>.git
git push -u origin main
```

## 2. Render Web Service 생성

Render Dashboard에서 `New` -> `Web Service`를 선택하고 GitHub 저장소를 연결합니다.

설정값:

```text
Language: Go
Branch: main
Build Command: go build -tags netgo -ldflags "-s -w" -o app ./cmd/server
Start Command: ./app
Health Check Path: /healthz
```

환경 변수:

```text
FILE_SHARE_DATA_DIR=data
MAX_UPLOAD_BYTES=10485760
```

Render가 `PORT` 환경 변수를 자동으로 주기 때문에 서버 코드는 그 포트로 실행됩니다.

## 3. 배포 후 BASE_URL 설정

첫 배포가 끝나면 `https://프로젝트명.onrender.com` 주소가 생깁니다. 그 주소를 Render 환경 변수에 추가합니다.

```text
BASE_URL=https://프로젝트명.onrender.com
```

환경 변수를 추가한 뒤 `Manual Deploy` 또는 `Redeploy`를 실행합니다.

## 4. 외부 접속 확인

브라우저:

```text
https://프로젝트명.onrender.com
```

curl 업로드:

```powershell
curl.exe -F "file=@sample.png" https://프로젝트명.onrender.com/upload
```

curl 다운로드:

```powershell
curl.exe -L "https://프로젝트명.onrender.com/share/<id>" -o downloaded-file
```

## 5. 저장소 주의점

무료 Web Service의 로컬 파일시스템은 재시작이나 재배포 때 사라질 수 있습니다. 이 과제에서 “외부에서 접속 가능하고 업로드/다운로드가 되는지 확인” 정도면 무료 배포로 시연할 수 있습니다.

파일을 오래 보존해야 하면 Render Persistent Disk 같은 영속 디스크를 붙이고 `FILE_SHARE_DATA_DIR`를 그 디스크 경로로 바꿔야 합니다.
