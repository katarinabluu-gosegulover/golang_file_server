// 이 파일은 애플리케이션의 진입점입니다.
// 실제 파일 업로드 로직은 internal/fileshare 패키지에 있고,
// main 패키지는 환경 설정을 읽고 HTTP 서버를 실행하는 역할만 합니다.
package main

import (
	"log"      // 서버 실행 상태와 치명적인 오류를 콘솔에 출력합니다.
	"net/http" // Go 표준 HTTP 서버를 사용합니다.
	"os"       // 환경 변수를 읽기 위해 사용합니다.
	"strconv"  // 문자열 환경 변수를 숫자로 변환하기 위해 사용합니다.
	"time"     // 서버 타임아웃 값을 지정하기 위해 사용합니다.

	"go-secure-file-share/internal/fileshare" // 우리가 만든 파일 공유 서버 패키지입니다.
)

func main() {
	// ADDR 환경 변수가 있으면 우선 사용하고, 없으면 배포 서비스가 주는 PORT를 사용합니다.
	addr := listenAddr()

	// FILE_SHARE_DATA_DIR 환경 변수가 있으면 그 디렉터리에 업로드 파일과 메타데이터를 저장합니다.
	dataDir := getenv("FILE_SHARE_DATA_DIR", "data")

	// BASE_URL은 배포된 공개 주소입니다. 설정하면 공유 링크 생성에 사용됩니다.
	baseURL := os.Getenv("BASE_URL")

	// MAX_UPLOAD_BYTES 환경 변수로 업로드 최대 크기를 바꿀 수 있고, 기본값은 10 MiB입니다.
	maxUploadBytes := getenvInt64("MAX_UPLOAD_BYTES", 10<<20)

	// 저장소 객체를 만듭니다. 여기서 data/uploads 디렉터리 생성과 metadata.json 로드가 일어납니다.
	store, err := fileshare.NewStore(dataDir)
	if err != nil {
		// 저장소를 만들 수 없으면 서버가 정상 동작할 수 없으므로 즉시 종료합니다.
		log.Fatalf("create store: %v", err)
	}

	// HTTP 요청을 처리할 핸들러를 만듭니다.
	handler := fileshare.NewServer(store, fileshare.ServerConfig{
		BaseURL:        baseURL,        // 외부 공유 링크에 사용할 공개 주소입니다.
		MaxUploadBytes: maxUploadBytes, // 업로드 크기 제한입니다.
	})

	// 표준 http.Server를 직접 구성해서 타임아웃을 명시합니다.
	server := &http.Server{
		Addr:              addr,             // 서버가 바인딩할 주소와 포트입니다.
		Handler:           handler,          // 요청을 처리할 라우터/핸들러입니다.
		ReadHeaderTimeout: 5 * time.Second,  // 헤더를 너무 오래 보내는 연결을 끊습니다.
		ReadTimeout:       30 * time.Second, // 요청 전체를 읽는 최대 시간입니다.
		WriteTimeout:      30 * time.Second, // 응답을 쓰는 최대 시간입니다.
		IdleTimeout:       60 * time.Second, // keep-alive 유휴 연결 유지 시간입니다.
	}

	// 로컬 실행 주소를 로그로 보여줍니다.
	log.Printf("listening on http://localhost%s", addr)
	if baseURL != "" {
		// BASE_URL이 설정된 경우 실제 공유 링크 기준 주소도 로그에 남깁니다.
		log.Printf("public base URL: %s", baseURL)
	}

	// 서버를 시작합니다. ListenAndServe가 오류를 반환하면 log.Fatal로 출력 후 종료합니다.
	log.Fatal(server.ListenAndServe())
}

func getenv(key, fallback string) string {
	// 지정한 환경 변수 값을 읽습니다.
	value := os.Getenv(key)
	if value == "" {
		// 환경 변수가 비어 있으면 기본값을 반환합니다.
		return fallback
	}
	// 환경 변수가 설정되어 있으면 그 값을 그대로 반환합니다.
	return value
}

func listenAddr() string {
	// ADDR은 로컬에서 ":8080"처럼 직접 바인딩 주소를 지정하고 싶을 때 사용합니다.
	if addr := os.Getenv("ADDR"); addr != "" {
		// ADDR이 있으면 PORT보다 우선합니다.
		return addr
	}

	// Render/Fly/Cloud Run 같은 배포 서비스는 보통 PORT 환경 변수를 제공합니다.
	if port := os.Getenv("PORT"); port != "" {
		if port[0] == ':' {
			// 사용자가 PORT=":10000"처럼 넣은 경우 그대로 사용합니다.
			return port
		}
		// net/http 서버 주소 형식에 맞게 "10000"을 ":10000"으로 바꿉니다.
		return ":" + port
	}

	// 아무 환경 변수도 없으면 로컬 개발 기본 포트 8080을 사용합니다.
	return ":8080"
}

func getenvInt64(key string, fallback int64) int64 {
	// 숫자로 해석할 환경 변수 값을 읽습니다.
	value := os.Getenv(key)
	if value == "" {
		// 값이 없으면 기본 숫자 값을 반환합니다.
		return fallback
	}

	// 문자열을 10진수 int64로 변환합니다.
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		// 변환 실패 또는 0 이하 값이면 위험한 설정으로 보지 않고 기본값을 사용합니다.
		return fallback
	}

	// 정상적인 양수 값이면 업로드 제한 값으로 사용합니다.
	return n
}
