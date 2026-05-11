// 이 파일은 업로드 파일과 메타데이터를 디스크에 저장하는 저장소 계층입니다.
package fileshare

import (
	"crypto/rand"   // 예측하기 어려운 공유 ID를 만들기 위해 사용합니다.
	"encoding/hex"  // 랜덤 바이트를 URL에 쓰기 쉬운 hex 문자열로 바꿉니다.
	"encoding/json" // metadata.json을 읽고 쓰기 위해 사용합니다.
	"errors"        // os.ErrExist 같은 오류 비교와 고정 오류 생성을 위해 사용합니다.
	"fmt"           // 값이 들어간 오류 메시지를 만들기 위해 사용합니다.
	"os"            // 디렉터리 생성, 파일 생성, 파일 읽기/쓰기에 사용합니다.
	"path/filepath" // 운영체제별 경로를 안전하게 조합하고 정규화합니다.
	"sort"          // 파일 목록을 업로드 최신순으로 정렬합니다.
	"strings"       // 경로 이탈 여부를 문자열 prefix로 확인할 때 사용합니다.
	"sync"          // 여러 요청이 동시에 metadata map에 접근해도 안전하게 보호합니다.
)

// Store는 업로드 파일의 실제 저장 위치와 메타데이터를 관리합니다.
type Store struct {
	mu        sync.RWMutex        // 동시에 읽기/쓰기가 들어와도 files map이 깨지지 않도록 잠급니다.
	dataDir   string              // 전체 데이터 디렉터리의 절대 경로입니다.
	uploadDir string              // 실제 업로드 파일이 저장되는 data/uploads 경로입니다.
	metaPath  string              // 메타데이터가 저장되는 metadata.json 경로입니다.
	files     map[string]FileMeta // 파일 ID를 key로 하는 메타데이터 map입니다.
}

func NewStore(dataDir string) (*Store, error) {
	// 상대 경로로 들어온 dataDir을 절대 경로로 바꿔 이후 경로 검증을 명확히 합니다.
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		// 절대 경로 계산 자체가 실패하면 저장소를 만들 수 없습니다.
		return nil, err
	}

	// Store 구조체를 초기화합니다.
	store := &Store{
		dataDir:   absDataDir,                                 // 데이터 디렉터리 절대 경로입니다.
		uploadDir: filepath.Join(absDataDir, "uploads"),       // 파일 저장 전용 하위 디렉터리입니다.
		metaPath:  filepath.Join(absDataDir, "metadata.json"), // 메타데이터 JSON 파일 경로입니다.
		files:     map[string]FileMeta{},                      // 메타데이터를 담을 빈 map입니다.
	}

	if err := os.MkdirAll(store.uploadDir, 0o700); err != nil {
		// 업로드 디렉터리를 만들 수 없으면 서버가 파일을 저장할 수 없습니다.
		return nil, err
	}
	if err := store.load(); err != nil {
		// 기존 metadata.json이 있으면 읽어 오고, 읽기 실패 시 서버 시작을 막습니다.
		return nil, err
	}

	// 초기화가 끝난 저장소를 반환합니다.
	return store, nil
}

func (s *Store) UploadDir() string {
	// 테스트나 디버깅에서 업로드 디렉터리 위치를 확인할 수 있게 반환합니다.
	return s.uploadDir
}

func (s *Store) Add(meta FileMeta) error {
	// metadata map을 수정할 것이므로 쓰기 잠금을 잡습니다.
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.files[meta.ID]; exists {
		// 랜덤 ID가 충돌하면 기존 메타데이터를 덮어쓰지 않고 오류를 냅니다.
		return fmt.Errorf("file id %q already exists", meta.ID)
	}

	// 새 파일 메타데이터를 메모리 map에 추가합니다.
	s.files[meta.ID] = meta

	// 메모리 상태를 metadata.json에 저장합니다.
	return s.saveLocked()
}

func (s *Store) Get(id string) (FileMeta, bool) {
	// 조회만 하므로 읽기 잠금을 잡습니다.
	s.mu.RLock()
	defer s.mu.RUnlock()

	// ID에 해당하는 메타데이터와 존재 여부를 함께 반환합니다.
	meta, ok := s.files[id]
	return meta, ok
}

func (s *Store) List() []FileMeta {
	// 파일 목록 조회 중 map이 바뀌지 않도록 읽기 잠금을 잡습니다.
	s.mu.RLock()
	defer s.mu.RUnlock()

	// map은 순서가 없으므로 slice로 복사해 정렬할 준비를 합니다.
	files := make([]FileMeta, 0, len(s.files))
	for _, meta := range s.files {
		// map 안의 각 메타데이터를 slice에 추가합니다.
		files = append(files, meta)
	}

	// 최신 업로드가 화면 위쪽에 보이도록 업로드 시각 내림차순으로 정렬합니다.
	sort.Slice(files, func(i, j int) bool {
		return files[i].UploadedAt.After(files[j].UploadedAt)
	})

	// 정렬된 파일 목록을 반환합니다.
	return files
}

func (s *Store) PathForStoredName(storedName string) (string, error) {
	if storedName == "" || filepath.IsAbs(storedName) || strings.ContainsAny(storedName, `/\`) {
		// 저장 파일명은 순수한 파일명이어야 하며 절대 경로나 경로 구분자를 허용하지 않습니다.
		return "", errors.New("invalid stored filename")
	}

	// 업로드 디렉터리와 저장 파일명을 결합합니다.
	path := filepath.Join(s.uploadDir, storedName)

	// 결합된 경로를 절대 경로로 정규화합니다.
	absPath, err := filepath.Abs(path)
	if err != nil {
		// 절대 경로 계산 실패 시 파일 접근을 중단합니다.
		return "", err
	}

	// 업로드 디렉터리 기준 상대 경로를 계산합니다.
	rel, err := filepath.Rel(s.uploadDir, absPath)
	if err != nil {
		// 상대 경로 계산 실패 시 안전하지 않은 상태로 보고 중단합니다.
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		// 상대 경로가 ..으로 시작하면 업로드 디렉터리 밖으로 나간 것이므로 차단합니다.
		return "", errors.New("stored path escapes upload directory")
	}

	// 업로드 디렉터리 내부에 있는 정규화된 절대 경로만 반환합니다.
	return absPath, nil
}

func (s *Store) NewUploadPath(ext string) (id string, storedName string, path string, file *os.File, err error) {
	for i := 0; i < 8; i++ {
		// 랜덤 ID 충돌은 매우 드물지만, 충돌하면 최대 8번까지 다시 시도합니다.
		id, err = randomHex(16)
		if err != nil {
			// 운영체제 랜덤 소스에서 읽기 실패하면 ID를 만들 수 없습니다.
			return "", "", "", nil, err
		}

		// 저장 파일명은 랜덤 ID와 검증된 확장자만 사용합니다.
		storedName = id + ext

		// 최종 파일 경로가 업로드 디렉터리 내부인지 검증합니다.
		path, err = s.PathForStoredName(storedName)
		if err != nil {
			return "", "", "", nil, err
		}

		// O_EXCL을 사용해 같은 이름의 파일이 이미 있으면 덮어쓰지 않습니다.
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			// ID 충돌이면 루프를 계속 돌며 새 ID를 만듭니다.
			continue
		}
		if err != nil {
			// 그 외 파일 생성 오류는 호출자에게 반환합니다.
			return "", "", "", nil, err
		}

		// 사용할 ID, 저장 파일명, 경로, 열린 파일 핸들을 반환합니다.
		return id, storedName, path, file, nil
	}

	// 여러 번 시도해도 고유 ID를 만들지 못하면 오류로 처리합니다.
	return "", "", "", nil, errors.New("could not allocate a unique file id")
}

func (s *Store) load() error {
	// metadata.json 파일을 읽습니다.
	data, err := os.ReadFile(s.metaPath)
	if errors.Is(err, os.ErrNotExist) {
		// 처음 실행이라 metadata.json이 없으면 빈 상태로 시작합니다.
		return nil
	}
	if err != nil {
		// 파일은 있지만 읽기 실패하면 오류를 반환합니다.
		return err
	}
	if len(data) == 0 {
		// 빈 파일이면 메타데이터가 없는 것으로 보고 넘어갑니다.
		return nil
	}

	// JSON 내용을 files map으로 역직렬화합니다.
	return json.Unmarshal(data, &s.files)
}

func (s *Store) saveLocked() error {
	// 임시 파일에 먼저 쓴 다음 rename해서 metadata.json이 반쯤 써지는 상황을 줄입니다.
	tmpPath := s.metaPath + ".tmp"

	// 메모리 map을 사람이 읽기 쉬운 JSON으로 변환합니다.
	data, err := json.MarshalIndent(s.files, "", "  ")
	if err != nil {
		// JSON 변환 실패 시 저장을 중단합니다.
		return err
	}
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		// 임시 파일 쓰기에 실패하면 오류를 반환합니다.
		return err
	}

	// 임시 파일을 실제 metadata.json으로 교체합니다.
	return os.Rename(tmpPath, s.metaPath)
}

func randomHex(size int) (string, error) {
	// 요청한 바이트 수만큼 버퍼를 만듭니다.
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		// 보안 랜덤 값을 읽지 못하면 ID 생성에 실패합니다.
		return "", err
	}

	// 랜덤 바이트를 hex 문자열로 바꿔 URL 경로에 안전하게 넣을 수 있게 합니다.
	return hex.EncodeToString(buf), nil
}

func (s *Store) Delete(id string) (FileMeta, bool, error) {
	// metadata map과 파일 삭제 순서를 보호하기 위해 쓰기 잠금을 잡습니다.
	s.mu.Lock()
	defer s.mu.Unlock()

	// 삭제할 ID의 메타데이터를 찾습니다.
	meta, ok := s.files[id]
	if !ok {
		// 메타데이터가 없으면 삭제할 대상이 없다는 뜻입니다.
		return FileMeta{}, false, nil
	}

	// 저장 파일명이 업로드 디렉터리 내부 경로인지 다시 검증합니다.
	path, err := s.PathForStoredName(meta.StoredName)
	if err != nil {
		return FileMeta{}, true, err
	}

	// 실제 업로드 파일을 디스크에서 삭제합니다.
	if err := os.Remove(path); err != nil {
		return FileMeta{}, true, err
	}

	// 메모리 metadata map에서도 삭제합니다.
	delete(s.files, id)

	// metadata.json에 삭제된 상태를 저장합니다.
	if err := s.saveLocked(); err != nil {
		return FileMeta{}, true, err
	}

	// 삭제한 파일 메타데이터와 성공 여부를 반환합니다.
	return meta, true, nil
}
