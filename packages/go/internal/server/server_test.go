package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/b4moss/es4/packages/go/internal/recovery"
	"github.com/b4moss/es4/packages/go/pkg/es4"
	"github.com/b4moss/es4/packages/go/pkg/options"
)

func newReadyServer(t *testing.T) (*Server, *es4.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{MemoryOnly: true, SnapshotInterval: 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if !db.Ready() {
		t.Fatal("expected Ready immediately for memory_only")
	}
	s := New(db)
	t.Cleanup(s.Close)
	return s, db
}

func doReq(t *testing.T, h http.Handler, method, path string, body []byte) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func readErrCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var body errorBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body.Error.Code
}

func TestHealth_Livez(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	resp := doReq(t, s.Handler(), http.MethodGet, "/livez", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestHealth_Livez_TrailingSlash_404(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	resp := doReq(t, s.Handler(), http.MethodGet, "/livez/", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want 404", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		t.Fatalf("unexpected redirect Location=%q", loc)
	}
}

func TestHealth_Readyz_AfterReady(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	resp := doReq(t, s.Handler(), http.MethodGet, "/readyz", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestHealth_Readyz_TrailingSlash_404(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	resp := doReq(t, s.Handler(), http.MethodGet, "/readyz/", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d want 404", resp.StatusCode)
	}
}

func TestState_CRUD_Clear(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	put := doReq(t, h, http.MethodPut, "/v1/keys/a/b", []byte(`{"x":1}`))
	put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("PUT status=%d", put.StatusCode)
	}

	get := doReq(t, h, http.MethodGet, "/v1/keys/a/b", nil)
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET status=%d", get.StatusCode)
	}
	if string(body) != `{"x":1}` {
		t.Fatalf("GET body=%s", body)
	}

	head := doReq(t, h, http.MethodHead, "/v1/keys/a/b", nil)
	hb, _ := io.ReadAll(head.Body)
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Fatalf("HEAD status=%d", head.StatusCode)
	}
	if len(hb) != 0 {
		t.Fatalf("HEAD body should be empty, got %q", hb)
	}

	del := doReq(t, h, http.MethodDelete, "/v1/keys/a/b", nil)
	del.Body.Close()
	if del.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status=%d", del.StatusCode)
	}

	get2 := doReq(t, h, http.MethodGet, "/v1/keys/a/b", nil)
	if get2.StatusCode != http.StatusNotFound {
		t.Fatalf("GET missing status=%d", get2.StatusCode)
	}
	if code := readErrCode(t, get2); code != CodeNotFound {
		t.Fatalf("code=%s", code)
	}

	head2 := doReq(t, h, http.MethodHead, "/v1/keys/a/b", nil)
	head2.Body.Close()
	if head2.StatusCode != http.StatusNotFound {
		t.Fatalf("HEAD missing status=%d", head2.StatusCode)
	}

	// single segment
	putA := doReq(t, h, http.MethodPut, "/v1/keys/a", []byte(`"ok"`))
	putA.Body.Close()
	if putA.StatusCode != http.StatusOK {
		t.Fatalf("PUT /a status=%d", putA.StatusCode)
	}
	getA := doReq(t, h, http.MethodGet, "/v1/keys/a", nil)
	ba, _ := io.ReadAll(getA.Body)
	getA.Body.Close()
	if getA.StatusCode != http.StatusOK || string(ba) != `"ok"` {
		t.Fatalf("GET /a status=%d body=%s", getA.StatusCode, ba)
	}

	_ = doReq(t, h, http.MethodPut, "/v1/keys/x", []byte(`1`)).Body.Close()
	_ = doReq(t, h, http.MethodPut, "/v1/keys/y", []byte(`2`)).Body.Close()
	clr := doReq(t, h, http.MethodPost, "/v1/clear", nil)
	clr.Body.Close()
	if clr.StatusCode != http.StatusOK {
		t.Fatalf("CLEAR status=%d", clr.StatusCode)
	}
	for _, k := range []string{"/v1/keys/a", "/v1/keys/x", "/v1/keys/y"} {
		g := doReq(t, h, http.MethodGet, k, nil)
		if g.StatusCode != http.StatusNotFound {
			t.Fatalf("after clear %s status=%d", k, g.StatusCode)
		}
		g.Body.Close()
	}
}

func TestState_Errors(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	get := doReq(t, h, http.MethodGet, "/v1/keys/missing", nil)
	if get.StatusCode != http.StatusNotFound || readErrCode(t, get) != CodeNotFound {
		t.Fatalf("GET missing")
	}
	del := doReq(t, h, http.MethodDelete, "/v1/keys/missing", nil)
	if del.StatusCode != http.StatusNotFound || readErrCode(t, del) != CodeNotFound {
		t.Fatalf("DELETE missing")
	}

	for _, path := range []string{"/v1/keys", "/v1/keys/", "/v1/keys/a/"} {
		resp := doReq(t, h, http.MethodGet, path, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status=%d want 400", path, resp.StatusCode)
		}
		if code := readErrCode(t, resp); code != CodeInvalidKey {
			t.Fatalf("%s code=%s", path, code)
		}
	}

	bad := doReq(t, h, http.MethodPut, "/v1/keys/a", []byte(`{not-json`))
	if bad.StatusCode != http.StatusBadRequest || readErrCode(t, bad) != CodeInvalidValue {
		t.Fatalf("invalid JSON")
	}
}

func TestState_TrailingSlash_FixedPaths(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/v1/"},
		{http.MethodPost, "/v1/clear/"},
	}
	for _, tc := range cases {
		resp := doReq(t, h, tc.method, tc.path, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s status=%d want 404", tc.method, tc.path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "" {
			t.Fatalf("redirect on %s", tc.path)
		}
	}
}

func TestTx_CommitRollback(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	// commit path
	begin := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var beginBody struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(begin.Body).Decode(&beginBody); err != nil {
		t.Fatal(err)
	}
	begin.Body.Close()
	if begin.StatusCode != http.StatusOK || beginBody.ID == "" {
		t.Fatalf("begin status=%d id=%q", begin.StatusCode, beginBody.ID)
	}
	id := beginBody.ID

	put := doReq(t, h, http.MethodPut, "/v1/tx/"+id+"/keys/k", []byte(`{"t":1}`))
	put.Body.Close()
	if put.StatusCode != http.StatusOK {
		t.Fatalf("tx PUT status=%d", put.StatusCode)
	}
	commit := doReq(t, h, http.MethodPost, "/v1/tx/"+id+"/commit", nil)
	commit.Body.Close()
	if commit.StatusCode != http.StatusOK {
		t.Fatalf("commit status=%d", commit.StatusCode)
	}
	outer := doReq(t, h, http.MethodGet, "/v1/keys/k", nil)
	ob, _ := io.ReadAll(outer.Body)
	outer.Body.Close()
	if outer.StatusCode != http.StatusOK || string(ob) != `{"t":1}` {
		t.Fatalf("outer after commit: %d %s", outer.StatusCode, ob)
	}

	// rollback path
	begin2 := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b2 struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin2.Body).Decode(&b2)
	begin2.Body.Close()
	id2 := b2.ID
	_ = doReq(t, h, http.MethodPut, "/v1/tx/"+id2+"/keys/r", []byte(`9`)).Body.Close()
	rb := doReq(t, h, http.MethodPost, "/v1/tx/"+id2+"/rollback", nil)
	rb.Body.Close()
	if rb.StatusCode != http.StatusOK {
		t.Fatalf("rollback status=%d", rb.StatusCode)
	}
	miss := doReq(t, h, http.MethodGet, "/v1/keys/r", nil)
	if miss.StatusCode != http.StatusNotFound {
		t.Fatalf("after rollback want 404, got %d", miss.StatusCode)
	}
	miss.Body.Close()
}

func TestTx_OpsInView(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	begin := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin.Body).Decode(&b)
	begin.Body.Close()
	id := b.ID
	base := "/v1/tx/" + id

	_ = doReq(t, h, http.MethodPut, base+"/keys/a", []byte(`1`)).Body.Close()
	_ = doReq(t, h, http.MethodPut, base+"/keys/b", []byte(`2`)).Body.Close()

	get := doReq(t, h, http.MethodGet, base+"/keys/a", nil)
	gb, _ := io.ReadAll(get.Body)
	get.Body.Close()
	if get.StatusCode != http.StatusOK || string(gb) != `1` {
		t.Fatalf("tx GET: %d %s", get.StatusCode, gb)
	}
	head := doReq(t, h, http.MethodHead, base+"/keys/a", nil)
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Fatalf("tx HEAD status=%d", head.StatusCode)
	}
	del := doReq(t, h, http.MethodDelete, base+"/keys/a", nil)
	del.Body.Close()
	if del.StatusCode != http.StatusOK {
		t.Fatalf("tx DELETE status=%d", del.StatusCode)
	}
	get2 := doReq(t, h, http.MethodGet, base+"/keys/a", nil)
	if get2.StatusCode != http.StatusNotFound || readErrCode(t, get2) != CodeNotFound {
		t.Fatalf("tx GET after delete")
	}
	headMiss := doReq(t, h, http.MethodHead, base+"/keys/a", nil)
	headMiss.Body.Close()
	if headMiss.StatusCode != http.StatusNotFound {
		t.Fatalf("tx HEAD miss status=%d", headMiss.StatusCode)
	}

	clr := doReq(t, h, http.MethodPost, base+"/clear", nil)
	clr.Body.Close()
	if clr.StatusCode != http.StatusOK {
		t.Fatalf("tx clear status=%d", clr.StatusCode)
	}
	getB := doReq(t, h, http.MethodGet, base+"/keys/b", nil)
	if getB.StatusCode != http.StatusNotFound {
		t.Fatalf("after tx clear want 404, got %d", getB.StatusCode)
	}
	getB.Body.Close()

	_ = doReq(t, h, http.MethodPost, base+"/rollback", nil).Body.Close()
}

func TestTx_Errors(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	begin := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin.Body).Decode(&b)
	begin.Body.Close()
	id := b.ID

	// nested begin
	nested := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	if nested.StatusCode != http.StatusConflict || readErrCode(t, nested) != CodeConflict {
		t.Fatalf("nested begin")
	}

	_ = doReq(t, h, http.MethodPost, "/v1/tx/"+id+"/commit", nil).Body.Close()

	// reuse after done
	reuse := doReq(t, h, http.MethodPut, "/v1/tx/"+id+"/keys/k", []byte(`1`))
	if reuse.StatusCode != http.StatusConflict || readErrCode(t, reuse) != CodeConflict {
		t.Fatalf("reuse after commit")
	}
	reuseCommit := doReq(t, h, http.MethodPost, "/v1/tx/"+id+"/commit", nil)
	if reuseCommit.StatusCode != http.StatusConflict || readErrCode(t, reuseCommit) != CodeConflict {
		t.Fatalf("reuse commit")
	}

	unknown := doReq(t, h, http.MethodPut, "/v1/tx/does-not-exist/keys/k", []byte(`1`))
	if unknown.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown tx status=%d", unknown.StatusCode)
	}
	unknown.Body.Close()

	// fresh tx for invalid key / value
	begin2 := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b2 struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin2.Body).Decode(&b2)
	begin2.Body.Close()
	id2 := b2.ID

	emptyKey := doReq(t, h, http.MethodGet, "/v1/tx/"+id2+"/keys", nil)
	if emptyKey.StatusCode != http.StatusBadRequest || readErrCode(t, emptyKey) != CodeInvalidKey {
		t.Fatalf("tx empty key")
	}
	badVal := doReq(t, h, http.MethodPut, "/v1/tx/"+id2+"/keys/k", []byte(`{`))
	if badVal.StatusCode != http.StatusBadRequest || readErrCode(t, badVal) != CodeInvalidValue {
		t.Fatalf("tx invalid value")
	}
	_ = doReq(t, h, http.MethodPost, "/v1/tx/"+id2+"/rollback", nil).Body.Close()
}

func TestTx_TrailingSlash(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	begin := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin.Body).Decode(&b)
	begin.Body.Close()
	id := b.ID

	paths := []string{
		"/v1/tx/",
		"/v1/tx/" + id + "/commit/",
		"/v1/tx/" + id + "/rollback/",
		"/v1/tx/" + id + "/clear/",
	}
	for _, p := range paths {
		resp := doReq(t, h, http.MethodPost, p, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d want 404", p, resp.StatusCode)
		}
	}
	_ = doReq(t, h, http.MethodPost, "/v1/tx/"+id+"/rollback", nil).Body.Close()
}

type blockingRecovery struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingRecovery) Save(context.Context, []byte) error { return nil }
func (r *blockingRecovery) Load(context.Context) ([]byte, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	return nil, recovery.ErrNotFound
}

func TestNotReady_StateAndTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	br := &blockingRecovery{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	db, err := es4.OpenWith(ctx, es4.OpenConfig{
		Options: options.Options{
			RestoreOnStartup: true,
			SnapshotInterval: 0,
			MemoryOnly:       false,
			RecoveryPath:     filepath.Join(t.TempDir(), "unused"),
		},
		Recovery: br,
	})
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(br.release) }) }
	t.Cleanup(func() {
		release()
		_ = db.Close()
	})

	select {
	case <-br.started:
	case <-time.After(2 * time.Second):
		t.Fatal("restore did not start")
	}
	if db.Ready() {
		t.Fatal("expected not ready")
	}

	s := New(db)
	t.Cleanup(s.Close)
	h := s.Handler()

	live := doReq(t, h, http.MethodGet, "/livez", nil)
	live.Body.Close()
	if live.StatusCode != http.StatusOK {
		t.Fatalf("livez=%d", live.StatusCode)
	}
	ready := doReq(t, h, http.MethodGet, "/readyz", nil)
	ready.Body.Close()
	if ready.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readyz=%d", ready.StatusCode)
	}

	checks := []struct {
		method, path string
		body         []byte
	}{
		{http.MethodPut, "/v1/keys/a", []byte(`1`)},
		{http.MethodGet, "/v1/keys/a", nil},
		{http.MethodDelete, "/v1/keys/a", nil},
		{http.MethodHead, "/v1/keys/a", nil},
		{http.MethodPost, "/v1/clear", nil},
		{http.MethodPost, "/v1/tx", nil},
		{http.MethodPut, "/v1/tx/x/keys/a", []byte(`1`)},
	}
	for _, tc := range checks {
		resp := doReq(t, h, tc.method, tc.path, tc.body)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s %s status=%d", tc.method, tc.path, resp.StatusCode)
		}
		if tc.method == http.MethodHead {
			resp.Body.Close()
			continue
		}
		if code := readErrCode(t, resp); code != CodeNotReady {
			t.Fatalf("%s %s code=%s", tc.method, tc.path, code)
		}
	}

	release()
	deadline := time.Now().Add(2 * time.Second)
	for !db.Ready() {
		if time.Now().After(deadline) {
			t.Fatal("never became ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ready2 := doReq(t, h, http.MethodGet, "/readyz", nil)
	ready2.Body.Close()
	if ready2.StatusCode != http.StatusOK {
		t.Fatalf("readyz after=%d", ready2.StatusCode)
	}
}

func TestTrailingSlash_RegressionTable(t *testing.T) {
	t.Parallel()
	s, _ := newReadyServer(t)
	h := s.Handler()

	begin := doReq(t, h, http.MethodPost, "/v1/tx", nil)
	var b struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(begin.Body).Decode(&b)
	begin.Body.Close()
	id := b.ID

	table := []struct {
		method, okPath, slashPath string
	}{
		{http.MethodGet, "/livez", "/livez/"},
		{http.MethodGet, "/readyz", "/readyz/"},
		{http.MethodPost, "/v1/clear", "/v1/clear/"},
		{http.MethodPost, "/v1/tx", "/v1/tx/"},
		{http.MethodPost, "/v1/tx/" + id + "/commit", "/v1/tx/" + id + "/commit/"},
		{http.MethodPost, "/v1/tx/" + id + "/rollback", "/v1/tx/" + id + "/rollback/"},
		{http.MethodPost, "/v1/tx/" + id + "/clear", "/v1/tx/" + id + "/clear/"},
	}
	// Fresh tx for commit/rollback/clear ok paths — use separate txs where needed.
	// First verify slash paths 404; ok paths for livez/readyz/clear already covered.
	for _, tc := range table {
		slash := doReq(t, h, tc.method, tc.slashPath, nil)
		slash.Body.Close()
		if slash.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s status=%d", tc.method, tc.slashPath, slash.StatusCode)
		}
	}
	_ = id
}

func TestListenAddr(t *testing.T) {
	t.Parallel()
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"port only", map[string]string{"PORT": "9090"}, ":9090"},
		{"listen only", map[string]string{"ES4_LISTEN_ADDR": ":7070"}, ":7070"},
		{"default", map[string]string{}, ":8080"},
		{"port wins", map[string]string{"PORT": "9090", "ES4_LISTEN_ADDR": ":7070"}, ":9090"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ListenAddr(env(tc.env))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	_, err := ListenAddr(env(map[string]string{"PORT": "abc"}))
	if err == nil {
		t.Fatal("want error for non-digit PORT")
	}
	_, err = ListenAddr(env(map[string]string{"PORT": "80a"}))
	if err == nil {
		t.Fatal("want error for mixed PORT")
	}
}

func TestLoadOptions_MemoryOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "es4.yaml")
	if err := os.WriteFile(cfg, []byte("memory_only: true\nsnapshot_interval: \"10s\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts, err := LoadOptions(func(k string) string {
		switch k {
		case "ES4_CONFIG_PATH":
			return cfg
		case "ES4_MEMORY_ONLY":
			return "" // empty skipped
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.MemoryOnly {
		t.Fatal("want memory_only from YAML")
	}
	// env overlay wins
	opts2, err := LoadOptions(func(k string) string {
		switch k {
		case "ES4_CONFIG_PATH":
			return cfg
		case "ES4_MEMORY_ONLY":
			return "false"
		case "ES4_RECOVERY_PATH":
			return filepath.Join(dir, "rp")
		case "ES4_SNAPSHOT_INTERVAL":
			return "0s"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts2.MemoryOnly {
		t.Fatal("env should overlay memory_only to false")
	}

	ctx := context.Background()
	db, err := es4.Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := New(db)
	t.Cleanup(s.Close)
	resp := doReq(t, s.Handler(), http.MethodPut, "/v1/keys/m", []byte(`true`))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("memory_only server PUT status=%d", resp.StatusCode)
	}
}

func TestDB_Ready_ImmediateWithoutRestore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := es4.Open(ctx, options.Options{RestoreOnStartup: false, SnapshotInterval: 0, RecoveryPath: filepath.Join(t.TempDir(), "x")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if !db.Ready() {
		t.Fatal("restore_on_startup=false should be Ready immediately")
	}
}
