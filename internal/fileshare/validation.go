// 이 파일은 업로드 입력값을 검증하는 함수들을 모아 둔 곳입니다.
package fileshare

import (
	"errors"        // 고정된 오류 메시지를 만들 때 사용합니다.
	"fmt"           // 값이 포함된 오류 메시지를 만들 때 사용합니다.
	"mime"          // Content-Type에서 media type만 분리하기 위해 사용합니다.
	"net/http"      // http.DetectContentType으로 파일 내용을 기반으로 MIME을 감지합니다.
	"path/filepath" // 파일 확장자를 안전하게 추출하기 위해 사용합니다.
	"strings"       // 문자열 공백 제거, 포함 여부 검사, 소문자 변환에 사용합니다.
)

// allowedMediaTypesByExt는 허용할 확장자와 그 확장자에 맞는 MIME 타입 목록입니다.
var allowedMediaTypesByExt = map[string]map[string]bool{
	".txt":  {"text/plain": true},                                            // 텍스트 파일은 text/plain만 허용합니다.
	".csv":  {"text/plain": true, "text/csv": true},                          // CSV는 환경에 따라 text/plain으로 감지될 수 있습니다.
	".json": {"application/json": true, "text/plain": true},                  // JSON도 작은 파일은 text/plain으로 감지될 수 있습니다.
	".pdf":  {"application/pdf": true},                                       // PDF 파일 시그니처와 맞는 타입입니다.
	".png":  {"image/png": true},                                             // PNG 이미지 타입입니다.
	".jpg":  {"image/jpeg": true},                                            // JPG 이미지 타입입니다.
	".jpeg": {"image/jpeg": true},                                            // JPEG 확장자도 JPG와 같은 MIME을 씁니다.
	".gif":  {"image/gif": true},                                             // GIF 이미지 타입입니다.
	".webp": {"image/webp": true},                                            // WebP 이미지 타입입니다.
	".zip":  {"application/zip": true, "application/x-zip-compressed": true}, // ZIP은 Windows에서 x-zip-compressed로 감지되기도 합니다.
	".md":   {"text/markdown": true, "text/plain": true},                     // Markdown도 작은 파일은 text/plain으로 감지될 수 있습니다.
}

func validateOriginalName(name string) (string, string, error) {
	// 파일명 앞뒤 공백을 제거해서 " file.txt " 같은 입력을 정리합니다.
	name = strings.TrimSpace(name)
	if name == "" {
		// 빈 파일명은 확장자와 저장 정책을 판단할 수 없으므로 거부합니다.
		return "", "", errors.New("filename is empty")
	}
	if len(name) > 255 {
		// 너무 긴 파일명은 파일시스템 호환성과 로그 가독성 문제를 만들 수 있어 거부합니다.
		return "", "", errors.New("filename is too long")
	}
	if strings.ContainsRune(name, 0) {
		// null byte는 일부 시스템에서 문자열 끝처럼 처리될 수 있어 차단합니다.
		return "", "", errors.New("filename contains a null byte")
	}
	for _, r := range name {
		// 파일명 안의 각 문자를 하나씩 검사합니다.
		if r < 32 || r == 127 {
			// 제어 문자는 로그/터미널/파일시스템에서 문제를 만들 수 있어 차단합니다.
			return "", "", errors.New("filename contains a control character")
		}
	}
	if strings.ContainsAny(name, `/\:`) {
		// 슬래시, 역슬래시, 콜론은 경로 또는 드라이브 문자로 해석될 수 있어 차단합니다.
		return "", "", errors.New("filename must not contain a path")
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") {
		// 현재/상위 디렉터리 표현과 숨김 파일 스타일 이름을 차단합니다.
		return "", "", errors.New("hidden or relative filenames are not allowed")
	}

	// 파일명에서 확장자를 추출하고 소문자로 통일합니다.
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		// 확장자가 없으면 허용 목록과 비교할 수 없으므로 거부합니다.
		return "", "", errors.New("filename must include an extension")
	}
	if _, ok := allowedMediaTypesByExt[ext]; !ok {
		// 허용 목록에 없는 확장자는 업로드를 막습니다.
		return "", "", fmt.Errorf("extension %q is not allowed", ext)
	}

	// 검증된 원본 파일명과 확장자를 호출자에게 반환합니다.
	return name, ext, nil
}

func detectMediaType(head []byte) string {
	// 파일 앞 512바이트 정도를 기반으로 Go 표준 라이브러리가 MIME 타입을 추정합니다.
	contentType := http.DetectContentType(head)

	// "text/plain; charset=utf-8" 같은 값에서 "text/plain"만 분리합니다.
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// 파싱에 실패하면 DetectContentType의 원본 결과를 그대로 반환합니다.
		return contentType
	}

	// 비교가 쉬워지도록 MIME 타입을 소문자로 통일합니다.
	return strings.ToLower(mediaType)
}

func validateDetectedType(ext, mediaType string) error {
	// 확장자에 대해 허용된 MIME 타입 목록을 가져옵니다.
	allowed := allowedMediaTypesByExt[ext]
	if !allowed[mediaType] {
		// 파일 내용으로 감지한 MIME이 확장자와 맞지 않으면 업로드를 거부합니다.
		return fmt.Errorf("detected MIME type %q does not match extension %q", mediaType, ext)
	}

	// 확장자와 MIME 조합이 허용되면 nil을 반환합니다.
	return nil
}

func validateShareID(id string) bool {
	// 이 서버의 공유 ID는 randomHex(16)으로 만든 32글자 hex 문자열입니다.
	if len(id) != 32 {
		// 길이가 다르면 정상 ID가 아니므로 false입니다.
		return false
	}
	for _, r := range id {
		// ID의 각 문자를 검사합니다.
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			// 0-9 또는 a-f이면 hex 문자이므로 다음 문자로 넘어갑니다.
			continue
		}
		// hex 범위를 벗어난 문자가 하나라도 있으면 잘못된 ID입니다.
		return false
	}

	// 길이와 문자 구성이 모두 맞으면 유효한 공유 ID 형식입니다.
	return true
}
