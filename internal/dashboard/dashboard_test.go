package dashboard

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noorshikalgar/loomless/internal/session"
)

// newTestServer isolates HOME so tests never touch real recordings and
// stubs revealPath so nothing opens a Finder window.
func newTestServer(t *testing.T) (http.Handler, *[]string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	var revealed []string
	orig := revealPath
	revealPath = func(path string, isFile bool) error {
		revealed = append(revealed, path)
		return nil
	}
	t.Cleanup(func() { revealPath = orig })
	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	return h, &revealed
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stoppedSession creates a finished recording with n screenshot steps.
func stoppedSession(t *testing.T, name string, shots int) string {
	t.Helper()
	s, dir, err := session.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < shots; i++ {
		if _, err := s.AddStep(dir, "step", "App", "Window", tinyPNG(t)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Stop(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestHomeRendersDesignSystemShell(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(h, "GET", "/", "")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-theme-pref="system"`, // server-rendered theme, no flash
		`/ui/tokens.css`,           // design system linked
		`class="sidebar"`,          // app shell
		`id="newForm"`,             // start panel when nothing is recording
		`Nothing recorded yet`,     // empty state
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home page missing %q", want)
		}
	}
}

func TestLibraryShowsThumbnailsForSessionsWithScreenshots(t *testing.T) {
	h, _ := newTestServer(t)
	stoppedSession(t, "with-shots", 2)
	stoppedSession(t, "no-shots", 0)
	body := do(h, "GET", "/sessions", "").Body.String()
	if !strings.Contains(body, `src="/thumb/with-shots"`) {
		t.Error("session with screenshots should render a thumbnail img")
	}
	if !strings.Contains(body, "No screenshots yet") {
		t.Error("session without screenshots should render the placeholder")
	}
	if !strings.Contains(body, "2 recordings") {
		t.Error("library header should count recordings")
	}
}

func TestThumbServesFirstAndLastScreenshot(t *testing.T) {
	h, _ := newTestServer(t)
	stoppedSession(t, "shots", 3)
	for _, path := range []string{"/thumb/shots", "/thumb/shots?pos=last"} {
		rec := do(h, "GET", path, "")
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "image/png") {
			t.Errorf("%s: status %d, type %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	stoppedSession(t, "empty", 0)
	if rec := do(h, "GET", "/thumb/empty", ""); rec.Code != 404 {
		t.Errorf("thumb for session without screenshots: status %d, want 404", rec.Code)
	}
	if rec := do(h, "GET", "/thumb/..%2F..%2Fetc", ""); rec.Code != 404 {
		t.Errorf("traversal thumb name: status %d, want 404", rec.Code)
	}
}

func TestDeleteMovesToTrashAndRestoreBringsItBack(t *testing.T) {
	h, _ := newTestServer(t)
	dir := stoppedSession(t, "doomed", 1)

	rec := do(h, "POST", "/api/sessions/doomed/delete", "")
	if rec.Code != 200 {
		t.Fatalf("delete status %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		TrashID string `json:"trash_id"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("deleted session still in sessions dir")
	}
	if strings.Contains(do(h, "GET", "/sessions", "").Body.String(), `data-name="doomed"`) {
		t.Fatal("deleted session still listed")
	}

	rec = do(h, "POST", "/api/trash/"+res.TrashID+"/restore", "")
	if rec.Code != 200 {
		t.Fatalf("restore status %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("restored session missing from sessions dir")
	}
}

func TestDeleteRefusesRecordingSession(t *testing.T) {
	h, _ := newTestServer(t)
	if _, _, err := session.Create("live"); err != nil { // never stopped = recording
		t.Fatal(err)
	}
	if rec := do(h, "POST", "/api/sessions/live/delete", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("delete recording session: status %d, want 400", rec.Code)
	}
}

func TestRevealSelectsExportedFileElseFolder(t *testing.T) {
	h, revealed := newTestServer(t)
	dir := stoppedSession(t, "rev", 1)
	if err := os.WriteFile(filepath.Join(dir, "rev.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	do(h, "POST", "/api/sessions/rev/reveal", `{"format":"md"}`)
	do(h, "POST", "/api/sessions/rev/reveal", `{"format":"pdf"}`) // not exported → folder
	do(h, "POST", "/api/sessions/rev/reveal", ``)
	want := []string{filepath.Join(dir, "rev.md"), dir, dir}
	if len(*revealed) != 3 {
		t.Fatalf("revealed %v", *revealed)
	}
	for i := range want {
		if (*revealed)[i] != want[i] {
			t.Errorf("reveal %d: got %q, want %q", i, (*revealed)[i], want[i])
		}
	}
}

func TestCreateRejectsTraversalName(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(h, "POST", "/api/sessions", `{"name":"../../escape"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestListAPIReportsLiveCounts(t *testing.T) {
	h, _ := newTestServer(t)
	stoppedSession(t, "a", 2)
	var list []summary
	if err := json.Unmarshal(do(h, "GET", "/api/sessions", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Steps != 2 || list[0].Screenshots != 2 || !list[0].HasThumb {
		t.Fatalf("unexpected list: %+v", list)
	}
}
