// 이 파일은 HTTP 라우팅, 업로드 처리, 다운로드 처리, JSON 응답을 담당합니다.
package fileshare

import (
	"bytes"          // JSON 파일 검증 후 다시 읽기 위해 메모리 Reader를 만들 때 사용합니다.
	"crypto/sha256"  // 업로드 파일의 무결성 해시를 계산합니다.
	"encoding/hex"   // SHA-256 바이트를 사람이 읽는 hex 문자열로 바꿉니다.
	"encoding/json"  // API 응답 JSON 인코딩과 JSON 업로드 검증에 사용합니다.
	"errors"         // io.EOF 같은 오류 비교와 고정 오류 생성을 위해 사용합니다.
	"fmt"            // 오류 메시지와 헤더 값을 포맷팅합니다.
	"html/template"  // 웹 UI HTML 템플릿을 안전하게 렌더링합니다.
	"io"             // 업로드 스트림 읽기, 복사, 다중 writer에 사용합니다.
	"mime/multipart" // multipart/form-data 업로드 파일 타입을 다룹니다.
	"net/http"       // HTTP 서버, 요청, 응답 처리를 담당합니다.
	"net/url"        // 리다이렉트 쿼리와 Content-Disposition 파일명 인코딩에 사용합니다.
	"os"             // 업로드 실패 시 생성된 파일을 삭제합니다.
	"strings"        // 문자열 비교, 헤더 판별, URL prefix 정리에 사용합니다.
	"time"           // 업로드 시각을 UTC로 저장합니다.
)

// defaultMaxUploadBytes는 환경 설정이 없을 때 사용할 기본 업로드 제한입니다.
const defaultMaxUploadBytes int64 = 10 << 20

// ServerConfig는 HTTP 서버 핸들러를 만들 때 외부에서 넣는 설정입니다.
type ServerConfig struct {
	BaseURL        string // 외부 공유 링크를 만들 때 사용할 공개 주소입니다.
	MaxUploadBytes int64  // 업로드 가능한 최대 파일 크기입니다.
}

// Server는 파일 공유 HTTP 서버가 동작하는 데 필요한 의존성과 설정을 묶습니다.
type Server struct {
	store          *Store             // 파일과 메타데이터를 저장하는 저장소입니다.
	baseURL        string             // 공유 링크 생성에 사용할 공개 URL prefix입니다.
	maxUploadBytes int64              // 요청 하나에서 허용할 최대 업로드 크기입니다.
	mux            *http.ServeMux     // URL 경로를 핸들러 함수로 연결하는 라우터입니다.
	tmpl           *template.Template // 메인 웹 UI를 렌더링할 HTML 템플릿입니다.
}

func NewServer(store *Store, cfg ServerConfig) http.Handler {
	// 설정에서 업로드 제한 값을 가져옵니다.
	maxUploadBytes := cfg.MaxUploadBytes
	if maxUploadBytes <= 0 {
		// 설정값이 없거나 잘못되었으면 기본 10 MiB 제한을 사용합니다.
		maxUploadBytes = defaultMaxUploadBytes
	}

	// 서버 구조체를 초기화합니다.
	s := &Server{
		store:          store,                                                                                                         // 디스크 저장소 의존성입니다.
		baseURL:        strings.TrimRight(cfg.BaseURL, "/"),                                                                           // 중복 슬래시 방지를 위해 끝의 /를 제거합니다.
		maxUploadBytes: maxUploadBytes,                                                                                                // 업로드 크기 제한입니다.
		mux:            http.NewServeMux(),                                                                                            // 표준 라이브러리 라우터입니다.
		tmpl:           template.Must(template.New("index").Funcs(template.FuncMap{"formatBytes": formatBytes}).Parse(indexTemplate)), // HTML 템플릿을 파싱합니다.
	}

	// URL 경로와 핸들러 함수를 등록합니다.
	s.routes()

	// 모든 응답에 보안 헤더를 붙이는 미들웨어로 mux를 감싼 뒤 반환합니다.
	return secureHeaders(s.mux)
}

func (s *Server) routes() {
	// GET / 는 브라우저용 업로드 UI와 파일 목록을 보여줍니다.
	s.mux.HandleFunc("GET /", s.handleIndex)

	// POST /upload 는 웹 UI와 curl 업로드를 모두 처리합니다.
	s.mux.HandleFunc("POST /upload", s.handleUpload)

	// GET /api/files 는 업로드 파일 목록을 JSON으로 반환합니다.
	s.mux.HandleFunc("GET /api/files", s.handleListFiles)

	// GET /share/{id} 는 공유 ID에 해당하는 파일을 다운로드합니다.
	s.mux.HandleFunc("GET /share/{id}", s.handleDownload)

	// GET /healthz 는 배포 환경에서 서버 상태 확인용으로 사용합니다.
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)

	// DELETE /share/{id} 는 공유 ID에 해당하는 파일과 메타데이터를 삭제합니다.
	s.mux.HandleFunc("DELETE /share/{id}", s.handleDelete)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// 저장소에서 업로드된 파일 목록을 가져옵니다.
	files := s.store.List()

	// 템플릿에 보여줄 파일 목록 slice를 미리 필요한 크기로 만듭니다.
	viewFiles := make([]viewFile, 0, len(files))
	for _, meta := range files {
		// 각 파일 메타데이터에 브라우저에서 사용할 절대 공유 URL을 붙입니다.
		viewFiles = append(viewFiles, viewFile{
			FileMeta: meta,                                // 기존 파일 메타데이터입니다.
			ShareURL: s.absoluteURL(r, "/share/"+meta.ID), // 현재 요청 기준의 공유 URL입니다.
		})
	}

	// HTML 템플릿에 넘길 화면 데이터를 구성합니다.
	data := indexView{
		Files:           viewFiles,                                                            // 화면에 표시할 파일 목록입니다.
		UploadedID:      r.URL.Query().Get("uploaded"),                                        // 업로드 성공 후 표시할 ID입니다.
		Error:           r.URL.Query().Get("error"),                                           // 업로드 실패 후 표시할 오류 메시지입니다.
		MaxUploadMB:     s.maxUploadBytes / (1 << 20),                                         // MiB 단위 업로드 제한입니다.
		AllowedExts:     ".txt, .csv, .json, .pdf, .png, .jpg, .jpeg, .gif, .webp, .zip, .md", // 화면에 보여줄 허용 확장자입니다.
		UploadEndpoint:  s.absoluteURL(r, "/upload"),                                          // curl 예시에 넣을 업로드 URL입니다.
		DownloadPattern: s.absoluteURL(r, "/share/{id}"),                                      // curl 예시에 넣을 다운로드 URL 패턴입니다.
		DeletePattern:   s.absoluteURL(r, "/share/{id}"),                                      // curl 예시에 넣을 삭제 URL 패턴입니다.
	}

	// 브라우저가 HTML로 해석하도록 Content-Type을 지정합니다.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		// 템플릿 렌더링 실패 시 500 오류를 반환합니다.
		http.Error(w, "render page", http.StatusInternalServerError)
	}
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 전체 요청 본문 크기를 제한합니다. multipart 여유분 1 MiB를 추가로 허용합니다.
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes+(1<<20))

	// multipart/form-data 요청을 파싱합니다.
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		// 폼 파싱 실패는 잘못된 클라이언트 요청이므로 400으로 응답합니다.
		s.writeUploadError(w, r, http.StatusBadRequest, "multipart form parsing failed")
		return
	}

	// name="file" 필드에서 업로드 파일과 헤더를 꺼냅니다.
	file, header, err := r.FormFile("file")
	if err != nil {
		// file 필드가 없으면 업로드 요청으로 인정하지 않습니다.
		s.writeUploadError(w, r, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// 파일명, MIME, 크기 검증을 거쳐 디스크에 저장합니다.
	response, err := s.saveUploadedFile(file, header)
	if err != nil {
		// 저장/검증 실패는 클라이언트에게 오류 메시지를 반환합니다.
		s.writeUploadError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// 응답에 절대 공유 URL을 채웁니다.
	response.ShareURL = s.absoluteURL(r, "/share/"+response.ID)

	// download_url도 같은 엔드포인트를 가리키도록 채웁니다.
	response.DownloadURL = response.ShareURL

	if wantsHTML(r) {
		// 브라우저 폼 업로드는 JSON 대신 메인 화면으로 리다이렉트합니다.
		http.Redirect(w, r, "/?uploaded="+url.QueryEscape(response.ID), http.StatusSeeOther)
		return
	}

	// curl/API 업로드는 JSON과 201 Created로 응답합니다.
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) saveUploadedFile(file multipart.File, header *multipart.FileHeader) (UploadResponse, error) {
	// 사용자가 보낸 원본 파일명을 검증하고 확장자를 가져옵니다.
	originalName, ext, err := validateOriginalName(header.Filename)
	if err != nil {
		// 파일명이나 확장자가 안전하지 않으면 업로드를 중단합니다.
		return UploadResponse{}, err
	}
	if header.Size <= 0 {
		// 빈 파일은 의미가 없고 MIME 검증도 애매하므로 거부합니다.
		return UploadResponse{}, errors.New("empty files are not allowed")
	}
	if header.Size > s.maxUploadBytes {
		// multipart 헤더에서 이미 제한 초과가 보이면 파일을 읽기 전에 거부합니다.
		return UploadResponse{}, fmt.Errorf("file exceeds the %s upload limit", formatBytes(s.maxUploadBytes))
	}

	// MIME 감지를 위해 파일 앞부분 최대 512바이트를 읽을 버퍼를 만듭니다.
	head := make([]byte, 512)

	// 업로드 파일에서 앞부분을 읽습니다.
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		// EOF가 아닌 읽기 오류는 업로드 실패로 처리합니다.
		return UploadResponse{}, err
	}
	if seeker, ok := file.(io.Seeker); ok {
		// MIME 감지를 위해 읽은 만큼 파일 포인터가 움직였으므로 다시 처음으로 되돌립니다.
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			// 되감기에 실패하면 전체 파일을 올바르게 저장할 수 없습니다.
			return UploadResponse{}, err
		}
	} else {
		// multipart.File은 보통 Seek이 가능하지만, 불가능한 구현이면 안전하게 거부합니다.
		return UploadResponse{}, errors.New("uploaded file stream cannot be rewound")
	}

	// 파일 내용으로 MIME 타입을 감지합니다.
	contentType := detectMediaType(head[:n])
	if err := validateDetectedType(ext, contentType); err != nil {
		// 확장자와 실제 내용의 MIME 타입이 맞지 않으면 업로드를 거부합니다.
		return UploadResponse{}, err
	}

	// 기본 저장 소스는 업로드 파일 스트림 자체입니다.
	var source io.Reader = file
	if ext == ".json" {
		// JSON은 MIME만으로 부족하므로 전체 내용을 읽어 문법 검증을 추가합니다.
		data, err := io.ReadAll(io.LimitReader(file, s.maxUploadBytes+1))
		if err != nil {
			// JSON 파일 전체 읽기에 실패하면 업로드를 중단합니다.
			return UploadResponse{}, err
		}
		if int64(len(data)) > s.maxUploadBytes {
			// 읽은 데이터가 제한보다 크면 업로드를 거부합니다.
			return UploadResponse{}, fmt.Errorf("file exceeds the %s upload limit", formatBytes(s.maxUploadBytes))
		}
		if !json.Valid(bytes.TrimSpace(data)) {
			// 공백을 제외한 JSON 내용이 문법적으로 올바른지 검사합니다.
			return UploadResponse{}, errors.New("json file is not valid JSON")
		}

		// 이미 전체 JSON을 읽었으므로 저장용 Reader를 메모리 데이터에서 다시 만듭니다.
		source = bytes.NewReader(data)

		// JSON 문법 검증까지 통과했으므로 응답 Content-Type을 application/json으로 고정합니다.
		contentType = "application/json"
	}

	// 저장소에서 랜덤 ID, 저장 파일명, 실제 경로, 열린 파일 핸들을 받습니다.
	id, storedName, storedPath, out, err := s.store.NewUploadPath(ext)
	if err != nil {
		// 저장 파일을 만들 수 없으면 업로드를 중단합니다.
		return UploadResponse{}, err
	}
	defer out.Close()

	// 업로드 파일의 SHA-256 해시를 계산할 hasher를 만듭니다.
	hasher := sha256.New()

	// 파일을 디스크와 hasher에 동시에 복사합니다.
	written, err := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(source, s.maxUploadBytes+1))
	if err != nil {
		// 복사 중 실패하면 불완전한 파일을 삭제합니다.
		_ = os.Remove(storedPath)
		return UploadResponse{}, err
	}
	if written > s.maxUploadBytes {
		// 실제로 읽은 바이트가 제한을 넘으면 파일을 삭제하고 거부합니다.
		_ = os.Remove(storedPath)
		return UploadResponse{}, fmt.Errorf("file exceeds the %s upload limit", formatBytes(s.maxUploadBytes))
	}
	if written != header.Size && header.Size > 0 {
		// multipart 헤더의 크기와 실제 저장 크기가 다르면 이상 상황으로 보고 삭제합니다.
		_ = os.Remove(storedPath)
		return UploadResponse{}, errors.New("uploaded file size changed while reading")
	}

	// SHA-256 결과를 hex 문자열로 바꿉니다.
	sum := hex.EncodeToString(hasher.Sum(nil))

	// metadata.json에 저장할 메타데이터를 구성합니다.
	meta := FileMeta{
		ID:           id,               // 공유 링크 ID입니다.
		OriginalName: originalName,     // 검증된 원본 파일명입니다.
		StoredName:   storedName,       // 서버 내부 저장 파일명입니다.
		Ext:          ext,              // 검증된 확장자입니다.
		ContentType:  contentType,      // 감지/검증된 MIME 타입입니다.
		Size:         written,          // 실제 저장된 크기입니다.
		SHA256:       sum,              // 파일 해시입니다.
		UploadedAt:   time.Now().UTC(), // 업로드 시각은 UTC로 저장합니다.
	}
	if err := s.store.Add(meta); err != nil {
		// 메타데이터 저장 실패 시 파일만 남지 않도록 삭제합니다.
		_ = os.Remove(storedPath)
		return UploadResponse{}, err
	}

	// API 응답에 들어갈 값을 반환합니다.
	return UploadResponse{
		ID:           meta.ID,           // 공유 ID입니다.
		OriginalName: meta.OriginalName, // 원본 파일명입니다.
		ContentType:  meta.ContentType,  // MIME 타입입니다.
		Size:         meta.Size,         // 파일 크기입니다.
		SHA256:       meta.SHA256,       // SHA-256 해시입니다.
	}, nil
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// URL의 {id} 부분에서 공유 ID를 꺼냅니다.
	id := r.PathValue("id")
	if !validateShareID(id) {
		// ID 형식이 틀리면 존재 여부를 알려주지 않고 404로 처리합니다.
		http.NotFound(w, r)
		return
	}

	// 저장소에서 해당 ID의 메타데이터를 찾습니다.
	meta, ok := s.store.Get(id)
	if !ok {
		// 정상 형식이지만 등록되지 않은 ID도 404입니다.
		http.NotFound(w, r)
		return
	}

	// 메타데이터의 저장 파일명이 안전한 경로인지 다시 검증하고 절대 경로를 얻습니다.
	storedPath, err := s.store.PathForStoredName(meta.StoredName)
	if err != nil {
		// 내부 데이터가 잘못된 경우 서버 오류로 처리합니다.
		http.Error(w, "invalid stored path", http.StatusInternalServerError)
		return
	}

	// 저장 당시 검증한 MIME 타입을 다운로드 응답 Content-Type으로 사용합니다.
	w.Header().Set("Content-Type", meta.ContentType)

	// 파일 크기를 명시해서 클라이언트가 다운로드 크기를 알 수 있게 합니다.
	w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))

	// 브라우저가 파일을 열기보다 다운로드하도록 attachment 헤더를 설정합니다.
	w.Header().Set("Content-Disposition", contentDispositionAttachment(meta.OriginalName))

	// 검증된 경로의 파일을 실제로 전송합니다.
	http.ServeFile(w, r, storedPath)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	// 목록 API 응답에서 FileMeta에 share_url을 추가하기 위한 지역 타입입니다.
	type listedFile struct {
		FileMeta        // 기존 메타데이터 필드를 JSON에 포함합니다.
		ShareURL string `json:"share_url"` // 클라이언트가 바로 쓸 공유 URL입니다.
	}

	// 저장소에서 파일 목록을 가져옵니다.
	files := s.store.List()

	// JSON 응답 slice를 필요한 크기로 준비합니다.
	result := make([]listedFile, 0, len(files))
	for _, meta := range files {
		// 각 파일 메타데이터에 절대 공유 URL을 붙입니다.
		result = append(result, listedFile{
			FileMeta: meta,                                // 파일 메타데이터입니다.
			ShareURL: s.absoluteURL(r, "/share/"+meta.ID), // 공유 다운로드 URL입니다.
		})
	}

	// 파일 목록을 JSON으로 반환합니다.
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// health check 응답은 단순 텍스트입니다.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// 서버가 살아 있음을 나타내는 ok를 반환합니다.
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	// URL의 {id} 부분에서 공유 ID를 꺼냅니다.
	id := r.PathValue("id")

	if !validateShareID(id) {
		// ID 형식이 틀리면 존재 여부를 알려주지 않고 404로 처리합니다.
		http.NotFound(w, r)
		return
	}

	// 저장소에서 해당 ID의 파일과 메타데이터를 삭제합니다.
	_, deleted, err := s.store.Delete(id)
	if err != nil {
		// 삭제 중 오류가 발생하면 서버 오류로 처리합니다.
		http.Error(w, "failed to delete file", http.StatusInternalServerError)
		return
	}
	if !deleted {
		// ID에 해당하는 파일이 없으면 404로 처리합니다.
		http.NotFound(w, r)
		return
	}

	// 성공적으로 삭제되었음을 알리는 응답을 반환합니다.
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) writeUploadError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsHTML(r) {
		// 브라우저 요청이면 오류 메시지를 쿼리로 붙여 메인 화면으로 돌려보냅니다.
		http.Redirect(w, r, "/?error="+url.QueryEscape(message), http.StatusSeeOther)
		return
	}

	// API/curl 요청이면 JSON 오류 응답을 반환합니다.
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) absoluteURL(r *http.Request, rawPath string) string {
	if s.baseURL != "" {
		// 배포 환경에서 BASE_URL이 설정되어 있으면 그 주소를 기준으로 공유 링크를 만듭니다.
		return s.baseURL + rawPath
	}

	// 기본 scheme은 로컬 개발 기준 http입니다.
	scheme := "http"
	if r.TLS != nil {
		// TLS 요청이면 https 링크를 만듭니다.
		scheme = "https"
	}
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto == "http" || forwardedProto == "https" {
		// 리버스 프록시 뒤에 있을 때 원래 프로토콜을 반영합니다.
		scheme = forwardedProto
	}

	// scheme, Host 헤더, path를 합쳐 절대 URL을 만듭니다.
	return scheme + "://" + r.Host + rawPath
}

func wantsHTML(r *http.Request) bool {
	// Accept 헤더를 읽어 브라우저 HTML 요청인지 판단합니다.
	accept := r.Header.Get("Accept")

	// text/html을 원하고 application/json을 명시하지 않으면 HTML 응답 흐름으로 봅니다.
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	// JSON 응답임을 명시합니다.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// HTTP 상태 코드를 먼저 씁니다.
	w.WriteHeader(status)

	// 값을 JSON으로 인코딩해 응답 본문에 씁니다.
	_ = json.NewEncoder(w).Encode(value)
}

func secureHeaders(next http.Handler) http.Handler {
	// http.HandlerFunc로 미들웨어를 만듭니다.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 브라우저가 Content-Type을 임의 추측하지 못하게 합니다.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// 다른 사이트가 iframe으로 이 페이지를 넣는 것을 막습니다.
		w.Header().Set("X-Frame-Options", "DENY")

		// 외부로 이동할 때 Referer 정보를 보내지 않도록 합니다.
		w.Header().Set("Referrer-Policy", "no-referrer")

		// 이 앱에서 허용할 리소스 출처를 현재 origin 중심으로 제한합니다.
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")

		// 보안 헤더를 붙인 뒤 다음 핸들러로 요청 처리를 넘깁니다.
		next.ServeHTTP(w, r)
	})
}

func contentDispositionAttachment(filename string) string {
	// 쌍따옴표는 Content-Disposition 헤더 구조를 깨뜨릴 수 있어 제거합니다.
	escaped := strings.ReplaceAll(filename, `"`, "")

	// ASCII filename과 UTF-8 filename*을 함께 넣어 다양한 클라이언트에서 파일명이 보이게 합니다.
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, escaped, url.PathEscape(filename))
}

func formatBytes(n int64) string {
	// 사람이 읽기 쉬운 크기 표기를 만들 때 1024 단위를 사용합니다.
	const unit = 1024
	if n < unit {
		// 1024바이트 미만은 B 단위로 그대로 표시합니다.
		return fmt.Sprintf("%d B", n)
	}

	// div는 현재 단위의 나눗셈 기준이고 exp는 단위 문자의 인덱스입니다.
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		// 1024 단위가 한 단계 커질 때마다 div와 exp를 증가시킵니다.
		div *= unit
		exp++
	}

	// KiB, MiB, GiB 같은 형식으로 소수점 한 자리까지 표시합니다.
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// indexView는 메인 HTML 템플릿에 전달되는 화면 데이터입니다.
type indexView struct {
	Files           []viewFile // 화면에 표시할 업로드 파일 목록입니다.
	UploadedID      string     // 업로드 성공 메시지를 표시할 때 사용하는 ID입니다.
	Error           string     // 업로드 실패 메시지를 표시할 때 사용하는 문자열입니다.
	MaxUploadMB     int64      // 화면에 표시할 업로드 제한 MiB 값입니다.
	AllowedExts     string     // 화면에 표시할 허용 확장자 목록입니다.
	UploadEndpoint  string     // curl 업로드 예시에 사용할 URL입니다.
	DownloadPattern string     // curl 다운로드 예시에 사용할 URL 패턴입니다.
	DeletePattern   string     // curl 삭제 예시에 사용할 URL 패턴입니다.
}

// viewFile은 HTML 화면 표시용 파일 메타데이터입니다.
type viewFile struct {
	FileMeta        // 기본 파일 메타데이터를 포함합니다.
	ShareURL string // 화면의 복사/다운로드 버튼에 사용할 공유 URL입니다.
}
