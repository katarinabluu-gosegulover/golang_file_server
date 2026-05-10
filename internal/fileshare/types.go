// fileshare 패키지는 업로드 파일 저장, 검증, 공유 링크 다운로드를 담당합니다.
package fileshare

import "time" // 업로드 시각을 저장하기 위해 사용합니다.

// FileMeta는 서버가 업로드 파일 하나에 대해 저장하는 메타데이터입니다.
type FileMeta struct {
	ID           string    `json:"id"`            // 공유 링크에 들어가는 32자리 랜덤 hex ID입니다.
	OriginalName string    `json:"original_name"` // 사용자가 업로드한 원본 파일명입니다.
	StoredName   string    `json:"stored_name"`   // 서버 내부에 실제 저장한 파일명입니다.
	Ext          string    `json:"ext"`           // 검증된 파일 확장자입니다.
	ContentType  string    `json:"content_type"`  // 서버가 파일 내용으로 감지한 MIME 타입입니다.
	Size         int64     `json:"size"`          // 저장된 파일 크기입니다.
	SHA256       string    `json:"sha256"`        // 파일 무결성 확인용 SHA-256 해시입니다.
	UploadedAt   time.Time `json:"uploaded_at"`   // UTC 기준 업로드 시각입니다.
}

// UploadResponse는 업로드 성공 시 웹 API가 클라이언트에게 돌려주는 JSON 구조입니다.
type UploadResponse struct {
	ID           string `json:"id"`            // 공유 파일 ID입니다.
	OriginalName string `json:"original_name"` // 사용자가 올린 원본 파일명입니다.
	ContentType  string `json:"content_type"`  // 감지 및 검증된 MIME 타입입니다.
	Size         int64  `json:"size"`          // 업로드된 파일 크기입니다.
	SHA256       string `json:"sha256"`        // 업로드된 파일의 SHA-256 해시입니다.
	ShareURL     string `json:"share_url"`     // 브라우저나 curl에서 접근할 공유 URL입니다.
	DownloadURL  string `json:"download_url"`  // ShareURL과 같은 다운로드 URL입니다.
}
