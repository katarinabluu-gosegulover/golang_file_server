// 이 파일은 브라우저에서 보이는 업로드 화면 HTML을 Go raw string으로 보관합니다.
package fileshare

// indexTemplate은 html/template 패키지가 렌더링할 메인 페이지 템플릿입니다.
// {{.Field}}와 {{range .Files}} 같은 부분은 Go 템플릿 문법입니다.
const indexTemplate = `<!doctype html>
<!-- HTML 문서가 최신 표준 모드로 렌더링되도록 선언합니다. -->
<html lang="ko">
<!-- 페이지 언어를 한국어로 지정해 브라우저와 보조 기술이 올바르게 해석하게 합니다. -->
<head>
  <!-- UTF-8 인코딩을 사용해 한국어와 파일명이 깨지지 않게 합니다. -->
  <meta charset="utf-8">
  <!-- 모바일 화면에서도 폭이 기기에 맞게 잡히도록 viewport를 설정합니다. -->
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <!-- 브라우저 탭에 표시될 제목입니다. -->
  <title>Secure File Share</title>
  <style>
    /* 전역 색상과 그림자 값을 CSS 변수로 정의합니다. */
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #1f2933;
      --muted: #657286;
      --line: #d9dee7;
      --accent: #1a7f64;
      --accent-strong: #12664f;
      --danger: #b42318;
      --info: #185abc;
      --shadow: 0 12px 32px rgba(28, 38, 55, 0.10);
    }

    /* 모든 요소의 width 계산에 padding과 border를 포함시킵니다. */
    * {
      box-sizing: border-box;
    }

    /* 전체 페이지 기본 배경, 글꼴, 글자색을 지정합니다. */
    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--text);
      font-family: Arial, "Noto Sans KR", sans-serif;
      letter-spacing: 0;
    }

    /* 본문 최대 폭과 좌우 여백을 정합니다. */
    main {
      width: min(1120px, calc(100% - 32px));
      margin: 0 auto;
      padding: 32px 0;
    }

    /* 페이지 제목 영역을 좌우 배치합니다. */
    .topbar {
      display: flex;
      align-items: end;
      justify-content: space-between;
      gap: 20px;
      margin-bottom: 20px;
    }

    /* 메인 제목 크기와 굵기를 지정합니다. */
    h1 {
      margin: 0;
      font-size: 30px;
      line-height: 1.15;
      font-weight: 700;
    }

    /* 제한 용량과 허용 확장자 안내 텍스트 스타일입니다. */
    .meta {
      margin: 8px 0 0;
      color: var(--muted);
      font-size: 14px;
    }

    /* 업로드 패널과 목록 패널을 2열 그리드로 배치합니다. */
    .shell {
      display: grid;
      grid-template-columns: minmax(280px, 380px) minmax(0, 1fr);
      gap: 20px;
      align-items: start;
    }

    /* 업로드/목록 영역에 공통 패널 스타일을 적용합니다. */
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }

    /* 업로드 패널 내부 여백입니다. */
    .upload {
      padding: 22px;
    }

    /* 패널 제목 크기와 아래 여백을 통일합니다. */
    .upload h2,
    .table-panel h2 {
      margin: 0 0 16px;
      font-size: 18px;
      line-height: 1.25;
    }

    /* 파일 선택 영역을 점선 박스로 보여줍니다. */
    .file-input {
      display: grid;
      gap: 10px;
      padding: 18px;
      border: 1px dashed #aeb8c7;
      border-radius: 8px;
      background: #fbfcfd;
    }

    /* 파일 input이 좁은 화면에서도 부모 폭을 넘지 않게 합니다. */
    input[type="file"] {
      width: 100%;
      min-height: 44px;
      font-size: 14px;
    }

    /* 보조 안내 문구 스타일입니다. */
    .hint {
      color: var(--muted);
      font-size: 13px;
      line-height: 1.45;
      overflow-wrap: anywhere;
    }

    /* 업로드 버튼 스타일입니다. */
    .button {
      width: 100%;
      min-height: 44px;
      margin-top: 14px;
      border: 0;
      border-radius: 6px;
      background: var(--accent);
      color: #fff;
      font-size: 15px;
      font-weight: 700;
      cursor: pointer;
    }

    /* 업로드 버튼 hover 상태입니다. */
    .button:hover {
      background: var(--accent-strong);
    }

    /* 성공/실패 메시지 공통 스타일입니다. */
    .message {
      margin-bottom: 16px;
      padding: 12px 14px;
      border-radius: 6px;
      font-size: 14px;
      line-height: 1.4;
    }

    /* 업로드 성공 메시지 색상입니다. */
    .message.ok {
      background: #e8f4ef;
      color: #0b5c46;
      border: 1px solid #b9dfd0;
    }

    /* 업로드 실패 메시지 색상입니다. */
    .message.error {
      background: #fff0ed;
      color: var(--danger);
      border: 1px solid #ffd2ca;
    }

    /* curl 사용 예시 박스입니다. */
    .api-box {
      margin-top: 18px;
      padding-top: 16px;
      border-top: 1px solid var(--line);
      display: grid;
      gap: 8px;
    }

    /* curl 명령어 code block 스타일입니다. */
    code {
      display: block;
      padding: 10px;
      border-radius: 6px;
      background: #17202a;
      color: #e7edf5;
      font-size: 12px;
      line-height: 1.45;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }

    /* 파일 목록 패널은 내부 테이블이 패널 테두리를 넘지 않게 숨깁니다. */
    .table-panel {
      overflow: hidden;
    }

    /* 파일 목록 제목과 파일 개수 표시 영역입니다. */
    .table-head {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 16px;
      padding: 20px 20px 12px;
    }

    /* 파일 목록 테이블 기본 스타일입니다. */
    table {
      width: 100%;
      border-collapse: collapse;
      table-layout: fixed;
    }

    /* 테이블 셀 공통 스타일입니다. */
    th,
    td {
      padding: 13px 20px;
      border-top: 1px solid var(--line);
      text-align: left;
      vertical-align: middle;
      font-size: 14px;
    }

    /* 테이블 헤더 스타일입니다. */
    th {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      font-weight: 700;
      background: #fbfcfd;
    }

    /* 파일명 텍스트 스타일입니다. */
    .name {
      font-weight: 700;
      overflow-wrap: anywhere;
    }

    /* MIME 타입과 ID를 보여주는 보조 텍스트입니다. */
    .sub {
      display: block;
      margin-top: 4px;
      color: var(--muted);
      font-size: 12px;
      overflow-wrap: anywhere;
    }

    /* 복사/다운로드 버튼을 오른쪽 정렬합니다. */
    .actions {
      display: flex;
      gap: 8px;
      justify-content: flex-end;
    }

    /* 공유 링크 복사 버튼, 다운로드 링크, 삭제 버튼의 공통 스타일입니다. */
    .link-button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 40px;
      min-height: 36px;
      padding: 0 10px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      color: var(--text);
      text-decoration: none;
      font-size: 13px;
      cursor: pointer;
    }

    /* 복사/다운로드 버튼 hover 상태입니다. */
    .link-button:hover {
      border-color: var(--info);
      color: var(--info);
    }

    /* 삭제 버튼은 위험한 동작이므로 붉은 계열로 구분합니다. */
    .danger-button {
      border-color: #f0b8b1;
      color: var(--danger);
    }

    /* 삭제 버튼 hover 상태입니다. */
    .danger-button:hover {
      border-color: var(--danger);
      color: var(--danger);
    }

    /* 업로드 파일이 없을 때 표시되는 빈 상태 스타일입니다. */
    .empty {
      padding: 36px 20px;
      color: var(--muted);
      text-align: center;
      border-top: 1px solid var(--line);
    }

    /* 모바일 화면에서는 2열 레이아웃을 1열로 바꿉니다. */
    @media (max-width: 820px) {
      main {
        width: min(100% - 20px, 720px);
        padding: 20px 0;
      }

      .topbar,
      .shell,
      .table-head {
        display: block;
      }

      .panel + .panel {
        margin-top: 16px;
      }

      th:nth-child(2),
      td:nth-child(2) {
        display: none;
      }

      th,
      td {
        padding: 12px;
      }

      .actions {
        flex-direction: column;
      }
    }
  </style>
</head>
<body>
  <!-- 본문 전체를 감싸는 중앙 정렬 컨테이너입니다. -->
  <main>
    <!-- 페이지 제목과 업로드 제한 안내를 담는 상단 영역입니다. -->
    <div class="topbar">
      <div>
        <!-- 서비스 이름을 표시합니다. -->
        <h1>Secure File Share</h1>
        <!-- Go 템플릿 값으로 최대 업로드 크기와 허용 확장자를 출력합니다. -->
        <p class="meta">업로드 제한 {{.MaxUploadMB}} MiB · 허용 확장자 {{.AllowedExts}}</p>
      </div>
    </div>

    <!-- 업로드 폼과 파일 목록을 나란히 담는 레이아웃입니다. -->
    <div class="shell">
      <!-- 파일 업로드 폼 패널입니다. -->
      <section class="panel upload" aria-labelledby="upload-title">
        {{if .UploadedID}}
          <!-- 업로드 성공 후 리다이렉트되면 성공 메시지를 표시합니다. -->
          <div class="message ok">공유 링크가 생성되었습니다.</div>
        {{end}}
        {{if .Error}}
          <!-- 업로드 실패 후 리다이렉트되면 오류 메시지를 표시합니다. -->
          <div class="message error">{{.Error}}</div>
        {{end}}

        <!-- 업로드 패널 제목입니다. -->
        <h2 id="upload-title">파일 업로드</h2>
        <!-- 브라우저 업로드는 /upload로 multipart/form-data POST 요청을 보냅니다. -->
        <form method="post" action="/upload" enctype="multipart/form-data">
          <!-- 파일 input과 안내 문구를 같은 클릭 영역으로 묶습니다. -->
          <label class="file-input">
            <!-- 서버의 r.FormFile("file")이 읽는 필드명입니다. -->
            <input type="file" name="file" required>
            <!-- 사용자가 서버의 저장 정책을 알 수 있게 짧게 안내합니다. -->
            <span class="hint">서버는 파일명을 저장 경로로 사용하지 않고, 감지된 MIME 타입과 확장자를 함께 검증합니다.</span>
          </label>
          <!-- 폼 제출 버튼입니다. -->
          <button class="button" type="submit">업로드</button>
        </form>

        <!-- curl 사용 예시를 보여주는 영역입니다. -->
        <div class="api-box" aria-label="curl examples">
          <!-- 업로드 curl 예시 라벨입니다. -->
          <span class="hint">curl 업로드</span>
          <!-- Go 템플릿 값으로 현재 서버의 업로드 URL을 출력합니다. -->
          <code>curl -F "file=@sample.png" {{.UploadEndpoint}}</code>
          <!-- 다운로드 curl 예시 라벨입니다. -->
          <span class="hint">curl 다운로드</span>
          <!-- Go 템플릿 값으로 다운로드 URL 패턴을 출력합니다. -->
          <code>curl -L "{{.DownloadPattern}}" -o downloaded-file</code>
          <!-- 삭제 curl 예시 라벨입니다. -->
          <span class="hint">curl 삭제</span>
          <!-- Go 템플릿 값으로 삭제 URL 패턴을 출력합니다. -->
          <code>curl -X DELETE "{{.DeletePattern}}"</code>
        </div>
      </section>

      <!-- 업로드된 공유 파일 목록 패널입니다. -->
      <section class="panel table-panel" aria-labelledby="files-title">
        <!-- 목록 제목과 파일 개수를 표시하는 헤더입니다. -->
        <div class="table-head">
          <!-- 파일 목록 제목입니다. -->
          <h2 id="files-title">공유 파일</h2>
          <!-- Go 템플릿의 len 함수로 파일 개수를 표시하고, 삭제 후 JavaScript가 갱신할 수 있게 data-count를 둡니다. -->
          <span class="hint" id="file-count" data-count="{{len .Files}}">{{len .Files}} files</span>
        </div>

        {{if .Files}}
          <!-- 업로드된 파일이 있으면 테이블을 렌더링합니다. -->
          <table>
            <thead>
              <tr>
                <!-- 파일명과 메타데이터 열입니다. -->
                <th style="width: 48%">파일</th>
                <!-- 파일 크기 열입니다. -->
                <th style="width: 18%">크기</th>
                <!-- 공유 액션 열입니다. -->
                <th style="width: 34%; text-align: right">공유</th>
              </tr>
            </thead>
            <tbody>
              {{range .Files}}
                <!-- 파일 하나당 한 행을 렌더링합니다. -->
                <tr data-file-row="{{.ID}}">
                  <td>
                    <!-- 원본 파일명을 표시합니다. -->
                    <span class="name">{{.OriginalName}}</span>
                    <!-- MIME 타입과 공유 ID를 보조 정보로 표시합니다. -->
                    <span class="sub">{{.ContentType}} · {{.ID}}</span>
                  </td>
                  <!-- formatBytes 함수로 사람이 읽기 쉬운 파일 크기를 표시합니다. -->
                  <td>{{formatBytes .Size}}</td>
                  <td>
                    <!-- 공유 링크 복사, 다운로드, 삭제 액션을 묶습니다. -->
                    <div class="actions">
                      <!-- data-copy에 공유 URL을 넣고 JavaScript가 클립보드에 복사합니다. -->
                      <button class="link-button" type="button" data-copy="{{.ShareURL}}">복사</button>
                      <!-- 공유 URL로 바로 다운로드할 수 있는 링크입니다. -->
                      <a class="link-button" href="{{.ShareURL}}">다운로드</a>
                      <!-- data-delete에 공유 URL을 넣고 JavaScript가 DELETE 요청을 보냅니다. -->
                      <button class="link-button danger-button" type="button" data-delete="{{.ShareURL}}" data-file-name="{{.OriginalName}}">삭제</button>
                    </div>
                  </td>
                </tr>
              {{end}}
            </tbody>
          </table>
        {{else}}
          <!-- 업로드된 파일이 없으면 빈 상태 문구를 보여줍니다. -->
          <div class="empty">아직 업로드된 파일이 없습니다.</div>
        {{end}}
      </section>
    </div>
  </main>

  <script>
    // data-copy 속성이 있는 모든 복사 버튼을 찾습니다.
    document.querySelectorAll("[data-copy]").forEach((button) => {
      // 각 버튼에 클릭 이벤트를 연결합니다.
      button.addEventListener("click", async () => {
        // 버튼의 data-copy 값, 즉 공유 URL을 클립보드에 씁니다.
        await navigator.clipboard.writeText(button.dataset.copy);
        // 버튼 원래 텍스트를 잠시 저장합니다.
        const previous = button.textContent;
        // 복사 성공 피드백으로 버튼 텍스트를 바꿉니다.
        button.textContent = "완료";
        // 1.2초 뒤 원래 버튼 텍스트로 되돌립니다.
        setTimeout(() => {
          // 저장해 둔 이전 텍스트를 복원합니다.
          button.textContent = previous;
        }, 1200);
      });
    });

    // data-delete 속성이 있는 모든 삭제 버튼을 찾습니다.
    document.querySelectorAll("[data-delete]").forEach((button) => {
      // 각 삭제 버튼에 클릭 이벤트를 연결합니다.
      button.addEventListener("click", async () => {
        // 실수로 삭제하지 않도록 브라우저 기본 확인창을 띄웁니다.
        if (!confirm(button.dataset.fileName + " 파일을 삭제할까요?")) {
          // 취소를 누르면 아무 요청도 보내지 않습니다.
          return;
        }

        // 삭제 요청 중에는 버튼을 비활성화해 중복 클릭을 막습니다.
        button.disabled = true;
        // 삭제 진행 상태를 버튼 텍스트로 표시합니다.
        button.textContent = "삭제 중";

        // 공유 URL로 DELETE 요청을 보냅니다.
        const response = await fetch(button.dataset.delete, { method: "DELETE" });
        if (!response.ok) {
          // 실패하면 사용자에게 알려주고 버튼을 다시 사용할 수 있게 합니다.
          alert("삭제에 실패했습니다.");
          button.disabled = false;
          button.textContent = "삭제";
          return;
        }

        // 삭제된 파일의 테이블 행을 찾습니다.
        const row = button.closest("tr");
        if (row) {
          // 서버 삭제가 성공했으면 화면에서도 행을 제거합니다.
          row.remove();
        }

        // 파일 개수 표시 요소를 찾습니다.
        const count = document.querySelector("#file-count");
        if (count) {
          // 현재 개수에서 1을 뺀 값을 계산합니다.
          const nextCount = Math.max(0, Number(count.dataset.count || "0") - 1);
          // 다음 삭제에도 쓸 수 있도록 data-count 값을 갱신합니다.
          count.dataset.count = String(nextCount);
          // 화면에 보이는 파일 개수 텍스트를 갱신합니다.
          count.textContent = nextCount + " files";

          // 마지막 파일을 삭제했으면 빈 상태 화면을 보여주기 위해 페이지를 새로고침합니다.
          if (nextCount === 0) {
            window.location.reload();
          }
        }
      });
    });
  </script>
</body>
</html>`
