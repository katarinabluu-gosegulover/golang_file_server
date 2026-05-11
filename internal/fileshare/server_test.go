// 이 파일은 업로드/다운로드 기능과 보안 검증이 깨지지 않는지 확인하는 테스트입니다.
package fileshare

import (
	"bytes"             // 업로드/다운로드 바이트 비교와 multipart body 생성에 사용합니다.
	"encoding/json"     // 업로드 JSON 응답을 구조체로 디코딩합니다.
	"mime/multipart"    // 테스트용 multipart/form-data 요청 본문을 만듭니다.
	"net/http"          // HTTP 메서드와 상태 코드 상수를 사용합니다.
	"net/http/httptest" // 실제 포트를 열지 않고 핸들러를 테스트합니다.
	"strings"           // 응답 헤더/본문에 특정 문자열이 있는지 확인합니다.
	"testing"           // Go 테스트 프레임워크입니다.
)

func TestUploadAndDownloadPNG(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// PNG 파일을 multipart/form-data 요청 본문으로 만듭니다.
	body, contentType := multipartBody(t, "file", "pixel.png", pngBytes())

	// POST /upload 요청을 생성합니다.
	req := httptest.NewRequest(http.MethodPost, "/upload", body)

	// multipart boundary가 포함된 Content-Type 헤더를 설정합니다.
	req.Header.Set("Content-Type", contentType)

	// 응답을 기록할 ResponseRecorder를 만듭니다.
	rec := httptest.NewRecorder()

	// 테스트 핸들러에 업로드 요청을 전달합니다.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		// 정상 업로드라면 201 Created가 나와야 합니다.
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// 업로드 응답 JSON을 담을 구조체입니다.
	var upload UploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &upload); err != nil {
		// JSON 응답을 파싱할 수 없으면 테스트 실패입니다.
		t.Fatalf("decode upload response: %v", err)
	}
	if upload.ID == "" || upload.ShareURL == "" {
		// 업로드 성공 응답에는 ID와 공유 URL이 반드시 있어야 합니다.
		t.Fatalf("upload response missing id/share URL: %#v", upload)
	}

	// 방금 받은 ID로 다운로드 요청을 만듭니다.
	downloadReq := httptest.NewRequest(http.MethodGet, "/share/"+upload.ID, nil)

	// 다운로드 응답을 기록할 recorder입니다.
	downloadRec := httptest.NewRecorder()

	// 테스트 핸들러에 다운로드 요청을 전달합니다.
	handler.ServeHTTP(downloadRec, downloadReq)

	if downloadRec.Code != http.StatusOK {
		// 정상 다운로드는 200 OK여야 합니다.
		t.Fatalf("expected 200, got %d", downloadRec.Code)
	}
	if got := downloadRec.Header().Get("Content-Type"); got != "image/png" {
		// PNG 파일로 감지된 Content-Type이 다운로드에도 유지되어야 합니다.
		t.Fatalf("expected image/png, got %q", got)
	}
	if !strings.Contains(downloadRec.Header().Get("Content-Disposition"), "attachment") {
		// 브라우저 실행보다 다운로드를 유도하기 위해 attachment가 있어야 합니다.
		t.Fatalf("expected attachment disposition, got %q", downloadRec.Header().Get("Content-Disposition"))
	}
	if !bytes.Equal(downloadRec.Body.Bytes(), pngBytes()) {
		// 다운로드한 바이트가 업로드한 원본 바이트와 같아야 합니다.
		t.Fatal("download body did not match uploaded content")
	}
}

func TestRejectsPathTraversalFilename(t *testing.T) {
	// 경로 조작을 시도하는 파일명 예시들입니다.
	tests := []string{"../pixel.png", `..\pixel.png`, `C:\temp\pixel.png`, "folder/pixel.png"}
	for _, name := range tests {
		// 각 파일명을 별도 subtest로 실행합니다.
		t.Run(name, func(t *testing.T) {
			if _, _, err := validateOriginalName(name); err == nil {
				// 경로 문자가 포함된 파일명은 반드시 거부되어야 합니다.
				t.Fatal("expected filename validation to reject path traversal")
			}
		})
	}
}

func TestRejectsDisallowedExtension(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// .exe 확장자는 허용 목록에 없으므로 거부되어야 합니다.
	body, contentType := multipartBody(t, "file", "tool.exe", []byte("MZ fake executable"))

	// 업로드 요청을 만듭니다.
	req := httptest.NewRequest(http.MethodPost, "/upload", body)

	// multipart Content-Type 헤더를 설정합니다.
	req.Header.Set("Content-Type", contentType)

	// 응답 기록기를 만듭니다.
	rec := httptest.NewRecorder()

	// 요청을 실행합니다.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		// 허용되지 않은 확장자는 400 Bad Request가 되어야 합니다.
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRejectsMIMEMismatch(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// 확장자는 .png지만 내용은 plain text인 파일을 만듭니다.
	body, contentType := multipartBody(t, "file", "not-image.png", []byte("plain text"))

	// 업로드 요청을 만듭니다.
	req := httptest.NewRequest(http.MethodPost, "/upload", body)

	// multipart Content-Type 헤더를 설정합니다.
	req.Header.Set("Content-Type", contentType)

	// 응답 기록기를 만듭니다.
	rec := httptest.NewRecorder()

	// 요청을 실행합니다.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		// MIME 불일치 파일은 400으로 거부되어야 합니다.
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MIME") {
		// 오류 메시지에 MIME 관련 원인이 포함되어야 디버깅하기 쉽습니다.
		t.Fatalf("expected MIME mismatch error, got %q", rec.Body.String())
	}
}

func TestRejectsInvalidJSON(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// 확장자는 .json이지만 문법이 깨진 JSON 내용을 만듭니다.
	body, contentType := multipartBody(t, "file", "broken.json", []byte(`{"missing":`))

	// 업로드 요청을 만듭니다.
	req := httptest.NewRequest(http.MethodPost, "/upload", body)

	// multipart Content-Type 헤더를 설정합니다.
	req.Header.Set("Content-Type", contentType)

	// 응답 기록기를 만듭니다.
	rec := httptest.NewRecorder()

	// 요청을 실행합니다.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		// JSON 문법이 깨진 파일은 400으로 거부되어야 합니다.
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRejectsInvalidShareID(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// 32자리 hex가 아닌 공유 ID로 다운로드 요청을 만듭니다.
	req := httptest.NewRequest(http.MethodGet, "/share/not-a-valid-id", nil)

	// 응답 기록기를 만듭니다.
	rec := httptest.NewRecorder()

	// 요청을 실행합니다.
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		// 잘못된 ID는 파일 존재 여부를 드러내지 않고 404로 처리합니다.
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteUploadedFile(t *testing.T) {
	// 테스트 전용 서버 핸들러를 만듭니다.
	handler := newTestHandler(t)

	// 삭제할 텍스트 파일을 먼저 업로드합니다.
	body, contentType := multipartBody(t, "file", "delete-me.txt", []byte("delete me"))

	// 업로드 요청을 만듭니다.
	uploadReq := httptest.NewRequest(http.MethodPost, "/upload", body)

	// multipart Content-Type 헤더를 설정합니다.
	uploadReq.Header.Set("Content-Type", contentType)

	// 업로드 응답 기록기를 만듭니다.
	uploadRec := httptest.NewRecorder()

	// 업로드 요청을 실행합니다.
	handler.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusCreated {
		// 삭제 테스트는 업로드 성공이 전제입니다.
		t.Fatalf("expected upload 201, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}

	// 업로드 응답 JSON을 담을 구조체입니다.
	var upload UploadResponse
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &upload); err != nil {
		// JSON 응답을 파싱할 수 없으면 테스트 실패입니다.
		t.Fatalf("decode upload response: %v", err)
	}

	// 업로드된 파일 ID로 DELETE 요청을 만듭니다.
	deleteReq := httptest.NewRequest(http.MethodDelete, "/share/"+upload.ID, nil)

	// 삭제 응답 기록기를 만듭니다.
	deleteRec := httptest.NewRecorder()

	// 삭제 요청을 실행합니다.
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		// 정상 삭제는 200 OK여야 합니다.
		t.Fatalf("expected delete 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	if !strings.Contains(deleteRec.Body.String(), `"deleted":true`) {
		// 삭제 성공 JSON 응답이 있어야 합니다.
		t.Fatalf("expected deleted response, got %q", deleteRec.Body.String())
	}

	// 같은 ID로 다시 다운로드 요청을 만듭니다.
	downloadReq := httptest.NewRequest(http.MethodGet, "/share/"+upload.ID, nil)

	// 다운로드 응답 기록기를 만듭니다.
	downloadRec := httptest.NewRecorder()

	// 다운로드 요청을 실행합니다.
	handler.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusNotFound {
		// 삭제된 파일은 다시 받을 수 없어야 합니다.
		t.Fatalf("expected deleted file to return 404, got %d", downloadRec.Code)
	}
}

func newTestHandler(t *testing.T) http.Handler {
	// helper 함수 실패 시 호출한 테스트 줄을 보여주도록 표시합니다.
	t.Helper()

	// 각 테스트마다 임시 디렉터리를 사용해 파일 저장 상태가 섞이지 않게 합니다.
	store, err := NewStore(t.TempDir())
	if err != nil {
		// 저장소 생성 실패는 테스트를 계속할 수 없는 오류입니다.
		t.Fatalf("new store: %v", err)
	}

	// 테스트용 BaseURL과 1 MiB 업로드 제한을 가진 서버 핸들러를 반환합니다.
	return NewServer(store, ServerConfig{BaseURL: "https://files.example.test", MaxUploadBytes: 1 << 20})
}

func multipartBody(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	// helper 함수 실패 시 호출한 테스트 줄을 보여주도록 표시합니다.
	t.Helper()

	// multipart 요청 본문을 담을 메모리 버퍼입니다.
	body := &bytes.Buffer{}

	// multipart/form-data writer를 만듭니다.
	writer := multipart.NewWriter(body)

	// 지정한 필드명과 파일명으로 파일 파트를 만듭니다.
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		// 파트 생성 실패는 테스트 준비 실패입니다.
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		// 파일 내용을 multipart 파트에 쓰지 못하면 테스트 준비 실패입니다.
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		// writer.Close가 boundary 끝을 쓰므로 반드시 성공해야 합니다.
		t.Fatalf("close writer: %v", err)
	}

	// 요청 본문 버퍼와 Content-Type 헤더 값을 함께 반환합니다.
	return body, writer.FormDataContentType()
}

func pngBytes() []byte {
	// MIME 감지가 image/png로 되는 최소 PNG 헤더 바이트입니다.
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature입니다.
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk 시작입니다.
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 이미지 크기입니다.
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, // 색상/압축 정보와 일부 CRC 바이트입니다.
		0x89, // 테스트용으로 충분한 마지막 바이트입니다.
	}
}
