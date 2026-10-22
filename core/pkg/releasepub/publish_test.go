package releasepub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Orama-Network/orama/core/pkg/autoupdate"
	"github.com/Orama-Network/orama/core/pkg/releaseverify"
)

const (
	testGiteaRepo  = "ICXCNIKA/orama"
	testGiteaToken = "test-gitea-token"
	// existingReleaseID and createdReleaseID tell a release the fake already
	// held from one it made on request.
	existingReleaseID = 7
	createdReleaseID  = 9
)

// eventLog is the order in which the fake Gitea and the rsync recorder were
// used, shared because the two run on different goroutines.
type eventLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *eventLog) add(line string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
}

func (l *eventLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// recorder is a Runner that remembers every rsync call; failing makes them fail.
type recorder struct {
	calls   []string
	failing bool
	log     *eventLog
}

func (r *recorder) run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, line)
	if r.log != nil {
		r.log.add("run " + name)
	}
	if r.failing {
		return []byte("boom"), errors.New("exit status 1")
	}
	return nil, nil
}

// upload is one archive the fake Gitea received.
type upload struct {
	releaseID string
	name      string
	filename  string
	content   []byte
}

// fakeGitea is a Gitea API with one repository whose release for a tag is
// there or not, as the test says.
type fakeGitea struct {
	t   *testing.T
	srv *httptest.Server
	log *eventLog

	exists       bool
	assets       []giteaAsset
	lookupStatus int // answers the lookup with this status instead
	failUpload   bool

	mu       sync.Mutex
	requests []string
	created  []map[string]any
	uploads  []upload
}

func newFakeGitea(t *testing.T) *fakeGitea {
	t.Helper()
	g := &fakeGitea{t: t, log: &eventLog{}, exists: true}
	g.srv = httptest.NewServer(http.HandlerFunc(g.handle))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitea) api() string { return g.srv.URL + "/api/v1" }

func (g *fakeGitea) handle(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	g.requests = append(g.requests, r.Method+" "+r.URL.RequestURI())
	g.mu.Unlock()
	g.log.add("gitea " + r.Method)
	if got := r.Header.Get("Authorization"); got != "token "+testGiteaToken {
		g.t.Errorf("Authorization = %q", got)
		http.Error(w, "bad credentials", http.StatusUnauthorized)
		return
	}
	base := "/api/v1/repos/" + testGiteaRepo + "/releases"
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(path, base+"/tags/"):
		g.lookup(w)
	case r.Method == http.MethodPost && path == base:
		g.create(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, base+"/") && strings.HasSuffix(path, "/assets"):
		g.receive(w, r, strings.TrimSuffix(strings.TrimPrefix(path, base+"/"), "/assets"))
	default:
		g.t.Errorf("unexpected request %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}
}

func (g *fakeGitea) lookup(w http.ResponseWriter) {
	switch {
	case g.lookupStatus != 0:
		http.Error(w, `{"message":"token is not valid"}`, g.lookupStatus)
	case !g.exists:
		http.Error(w, `{"message":"release does not exist"}`, http.StatusNotFound)
	default:
		g.reply(w, http.StatusOK, map[string]any{"id": existingReleaseID, "assets": g.assets})
	}
}

func (g *fakeGitea) create(w http.ResponseWriter, r *http.Request) {
	var doc map[string]any
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		g.t.Errorf("create body: %v", err)
	}
	g.mu.Lock()
	g.created = append(g.created, doc)
	g.mu.Unlock()
	g.reply(w, http.StatusCreated, map[string]any{"id": createdReleaseID})
}

func (g *fakeGitea) receive(w http.ResponseWriter, r *http.Request, releaseID string) {
	if g.failUpload {
		http.Error(w, "storage is full", http.StatusInternalServerError)
		return
	}
	file, header, err := r.FormFile("attachment")
	if err != nil {
		g.t.Errorf("no attachment in the upload: %v", err)
		http.Error(w, "no attachment", http.StatusBadRequest)
		return
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		g.t.Errorf("read the attachment: %v", err)
	}
	g.mu.Lock()
	g.uploads = append(g.uploads, upload{releaseID: releaseID, name: r.URL.Query().Get("name"), filename: header.Filename, content: content})
	g.mu.Unlock()
	g.log.add("gitea upload")
	g.reply(w, http.StatusCreated, map[string]any{"id": 1})
}

func (g *fakeGitea) reply(w http.ResponseWriter, status int, doc map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		g.t.Errorf("reply: %v", err)
	}
}

func (g *fakeGitea) requestCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.requests)
}

func (g *fakeGitea) uploaded() []upload {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]upload(nil), g.uploads...)
}

func cutOne(t *testing.T, repo Repo, agent *fakeAgent, channel, version, content string) string {
	t.Helper()
	path := archive(t, version, "amd64", content)
	if _, err := Cut(t.Context(), cutParams(repo, agent, mustChannel(t, channel), path)); err != nil {
		t.Fatal(err)
	}
	return path
}

func publishParams(repo Repo, r *recorder, g *fakeGitea) PublishParams {
	r.log = g.log
	return PublishParams{
		Repo: repo, GiteaRepo: testGiteaRepo, GiteaAPI: g.api(), GiteaToken: testGiteaToken,
		MetadataDest: "releases.example.org:/srv/releases/", Now: testNow, Run: r.run, HTTP: g.srv.Client(),
	}
}

func TestPublish_uploadsArchivesFirstThenMetadataWithTheTimestampLast(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g, rec := newFakeGitea(t), &recorder{}

	if err := Publish(t.Context(), publishParams(repo, rec, g)); err != nil {
		t.Fatal(err)
	}
	wantLookup := "GET /api/v1/repos/" + testGiteaRepo + "/releases/tags/release-nightly-0.3.1"
	if len(g.requests) != 2 || g.requests[0] != wantLookup {
		t.Fatalf("requests = %q", g.requests)
	}
	wantUpload := fmt.Sprintf("POST /api/v1/repos/%s/releases/%d/assets?name=%s", testGiteaRepo, existingReleaseID, filepath.Base(path))
	if g.requests[1] != wantUpload {
		t.Fatalf("upload request = %q, want %q", g.requests[1], wantUpload)
	}
	ups := g.uploaded()
	if len(ups) != 1 || ups[0].filename != filepath.Base(path) || !bytes.Equal(ups[0].content, mustRead(t, path)) {
		t.Fatalf("uploads = %+v", ups)
	}
	if got := strings.Join(g.log.all(), ";"); got != "gitea GET;gitea POST;gitea upload;run rsync;run rsync" {
		t.Fatalf("order = %s", got)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("rsync calls = %q", rec.calls)
	}
	first, last := rec.calls[0], rec.calls[1]
	for _, f := range []string{"1.root.json", "root.json", "targets.json", "snapshot.json"} {
		if !strings.Contains(first, "/"+f) {
			t.Errorf("the first rsync lacks %s: %s", f, first)
		}
	}
	if strings.Contains(first, "timestamp.json") || !strings.HasPrefix(last, "rsync") || !strings.Contains(last, "/timestamp.json releases.example.org:/srv/releases/") {
		t.Fatalf("the timestamp is not alone and last:\n%s\n%s", first, last)
	}
	if strings.Contains(first, PendingFile) || strings.Contains(last, PendingFile) {
		t.Fatal("the pending record was sent to the host")
	}
	if p, _ := repo.ReadPending(); p != nil {
		t.Fatal("the pending record outlived the upload")
	}
}

func TestPublish_createsTheGiteaReleaseWhenThereIsNone(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g := newFakeGitea(t)
	g.exists = false

	if err := Publish(t.Context(), publishParams(repo, &recorder{}, g)); err != nil {
		t.Fatal(err)
	}
	if len(g.created) != 1 {
		t.Fatalf("created = %v", g.created)
	}
	doc := g.created[0]
	if doc["tag_name"] != "release-nightly-0.3.1" || doc["name"] != "nightly 0.3.1" || doc["prerelease"] != true || doc["draft"] != false {
		t.Fatalf("create = %v", doc)
	}
	if body, _ := doc["body"].(string); !strings.Contains(body, "Orama nightly 0.3.1") {
		t.Fatalf("notes = %q", body)
	}
	ups := g.uploaded()
	if len(ups) != 1 || ups[0].releaseID != fmt.Sprint(createdReleaseID) || ups[0].name != filepath.Base(path) {
		t.Fatalf("the archive did not go to the created release: %+v", ups)
	}
}

func TestPublish_aMainReleaseIsNotAPrerelease(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "main", "0.3.0", "bytes")
	g := newFakeGitea(t)
	g.exists = false
	if err := Publish(t.Context(), publishParams(repo, &recorder{}, g)); err != nil {
		t.Fatal(err)
	}
	if len(g.created) != 1 || g.created[0]["prerelease"] != false || g.created[0]["name"] != "main 0.3.0" {
		t.Fatalf("created = %v", g.created)
	}
}

func TestPublish_anyOtherGiteaLookupFailureStopsBeforeAnythingIsSent(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g, rec := newFakeGitea(t), &recorder{}
	g.lookupStatus = http.StatusUnauthorized

	err := Publish(t.Context(), publishParams(repo, rec, g))
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "release-nightly-0.3.1") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), testGiteaToken) {
		t.Fatal("the token is in the error")
	}
	if len(rec.calls) != 0 || len(g.created) != 0 || len(g.uploaded()) != 0 {
		t.Fatalf("something was sent: rsync %q, created %v, uploads %d", rec.calls, g.created, len(g.uploaded()))
	}
	if p, _ := repo.ReadPending(); p == nil {
		t.Fatal("the pending record was cleared by a failed publish")
	}
}

func TestPublish_aFailedUploadOrRsyncIsAnError(t *testing.T) {
	for _, failing := range []string{"upload", "rsync"} {
		agent := newFakeAgent(t)
		repo := newRepo(t, agent)
		cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
		g, rec := newFakeGitea(t), &recorder{failing: failing == "rsync"}
		g.failUpload = failing == "upload"
		err := Publish(t.Context(), publishParams(repo, rec, g))
		if err == nil {
			t.Errorf("a failing %s was ignored", failing)
		}
		if failing == "upload" && (!strings.Contains(fmt.Sprint(err), "500") || len(rec.calls) != 0) {
			t.Errorf("a failed upload: err = %v, rsync %q", err, rec.calls)
		}
		if p, _ := repo.ReadPending(); p == nil {
			t.Errorf("%s: the pending record was cleared by a failed publish", failing)
		}
	}
}

func TestPublish_anArchiveChangedAfterTheCutIsNotUploaded(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	if err := os.WriteFile(path, []byte("swapped"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, rec := newFakeGitea(t), &recorder{}
	if err := Publish(t.Context(), publishParams(repo, rec, g)); err == nil || !strings.Contains(err.Error(), "changed after the cut") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.calls) != 0 || g.requestCount() != 0 {
		t.Fatalf("something ran: rsync %q, %d requests", rec.calls, g.requestCount())
	}
}

func TestPublish_metadataThatWouldNotVerifyIsNotSent(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	if err := os.WriteFile(filepath.Join(repo.Dir, TargetsFile), []byte(`{"signed":{},"signatures":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	g, rec := newFakeGitea(t), &recorder{}
	if err := Publish(t.Context(), publishParams(repo, rec, g)); err == nil || len(rec.calls) != 0 || g.requestCount() != 0 {
		t.Fatalf("err = %v, rsync %q, %d requests", err, rec.calls, g.requestCount())
	}
	p := publishParams(repo, rec, g)
	p.Now = testNow.Add(30 * 24 * time.Hour)
	if err := Publish(t.Context(), p); err == nil {
		t.Fatal("an expired timestamp was published")
	}
	if g.requestCount() != 0 {
		t.Fatalf("an expired timestamp reached Gitea: %q", g.requests)
	}
}

func TestPublish_aRefreshUploadsMetadataOnly(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	if err := Publish(t.Context(), publishParams(repo, &recorder{}, newFakeGitea(t))); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshTimestamp(t.Context(), agent, repo, mustChannel(t, "nightly"), testNow.Add(time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	g, rec := newFakeGitea(t), &recorder{}
	if err := Publish(t.Context(), publishParams(repo, rec, g)); err != nil {
		t.Fatal(err)
	}
	if g.requestCount() != 0 {
		t.Fatalf("a refresh touched Gitea: %q", g.requests)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("the metadata was not sent: %q", rec.calls)
	}
}

func TestPublish_dryRunPrintsAPlanAndSendsNothing(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	var out bytes.Buffer
	g, rec := newFakeGitea(t), &recorder{}
	p := publishParams(repo, rec, g)
	p.DryRun, p.Progress = true, &out

	if err := Publish(t.Context(), p); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"release-nightly-0.3.1", path, filepath.Base(path), "/releases/tags/release-nightly-0.3.1", "rsync -a", "timestamp.json releases.example.org"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run output lacks %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), testGiteaToken) || strings.Contains(strings.ToLower(out.String()), "authorization") {
		t.Fatalf("a credential is in the plan:\n%s", out.String())
	}
	if g.requestCount() != 0 || len(rec.calls) != 0 {
		t.Fatalf("a dry run sent something: %d requests, rsync %q", g.requestCount(), rec.calls)
	}
	if pending, _ := repo.ReadPending(); pending == nil {
		t.Fatal("a dry run cleared the pending record")
	}
}

// What publish sends, served as the release host serves it, is what the
// clients' fetch path (root rotation, then the channel's metadata, then the
// archive) accepts: the whole chain from a cut to an install.
func TestRelease_whatIsPublishedIsWhatAClientFetchesAndVerifies(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	archivePath := cutOne(t, repo, agent, "nightly", "0.3.1", "the nightly archive")
	content := string(mustRead(t, archivePath))
	if _, err := RenewRoot(t.Context(), agent, repo, testNow.Add(24*time.Hour), nil); err != nil {
		t.Fatal(err)
	}
	root1 := mustRead(t, filepath.Join(repo.Dir, "1.root.json"))

	served := t.TempDir()
	copyDir(t, repo.Dir, served, TargetsFile, SnapshotFile, TimestampFile, RootFile, "1.root.json", "2.root.json")
	servedArchive := filepath.Join(served, "targets", "nightly", "orama-0.3.1-linux-amd64.tar.gz")
	if err := os.MkdirAll(filepath.Dir(servedArchive), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(servedArchive, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseverify.AllowLocalRepositories(t)
	srv := httptest.NewServer(http.FileServer(http.Dir(served)))
	t.Cleanup(srv.Close)

	// The node holds the first root; the repository has since renewed it.
	node := t.TempDir()
	rootPath := filepath.Join(node, "release-root.json")
	if err := os.WriteFile(rootPath, root1, 0o644); err != nil {
		t.Fatal(err)
	}
	src := autoupdate.Source{RootPath: rootPath, SeenPath: filepath.Join(node, "seen.json"), WorkDir: filepath.Join(node, "work"), Arch: "amd64", Now: func() time.Time { return testNow.Add(time.Hour) }}
	rel, ok, err := src.Newest(t.Context(), srv.URL, "nightly")
	if err != nil || !ok {
		t.Fatalf("Newest: ok=%v err=%v", ok, err)
	}
	if err := src.Download(t.Context(), srv.URL, rel); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got := mustRead(t, rel.ArchivePath())
	if string(got) != content || rel.Version != "0.3.1" {
		t.Fatalf("version %s, archive %q", rel.Version, got)
	}
	if !bytes.Equal(mustRead(t, rootPath), mustRead(t, filepath.Join(repo.Dir, "2.root.json"))) {
		t.Fatal("the node did not follow the root renewal")
	}
}

func copyDir(t *testing.T, from, to string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(to, n), mustRead(t, filepath.Join(from, n)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPublish_aPublishThatStoppedAfterTheUploadCanBeRunAgain(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g, rec := newFakeGitea(t), &recorder{}
	g.assets = []giteaAsset{{Name: filepath.Base(path), Size: mustStat(t, path)}}

	if err := Publish(t.Context(), publishParams(repo, rec, g)); err != nil {
		t.Fatal(err)
	}
	if len(g.uploaded()) != 0 || len(g.requests) != 1 {
		t.Fatalf("an archive the release already holds was uploaded again: %q", g.requests)
	}
	if len(rec.calls) != 2 || !strings.HasPrefix(rec.calls[1], "rsync") {
		t.Fatalf("the metadata was not sent: %q", rec.calls)
	}
}

func TestPublish_anArchiveOfThatNameWithOtherBytesIsNeverOverwritten(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	path := cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g, rec := newFakeGitea(t), &recorder{}
	g.assets = []giteaAsset{{Name: filepath.Base(path), Size: 1}}

	if err := Publish(t.Context(), publishParams(repo, rec, g)); err == nil || !strings.Contains(err.Error(), "never overwritten") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.calls) != 0 || len(g.uploaded()) != 0 {
		t.Fatalf("something was sent: rsync %q, uploads %d", rec.calls, len(g.uploaded()))
	}
	if p, _ := repo.ReadPending(); p == nil {
		t.Fatal("the pending record was cleared by a refused publish")
	}
}

func TestPublish_aTamperedPendingRecordOrAnOptionLookingDestinationIsRefused(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	good := mustRead(t, filepath.Join(repo.Dir, PendingFile))
	cases := map[string]func(p *PublishParams) string{
		"a tag that is not the cut's": func(*PublishParams) string {
			return strings.Replace(string(good), "release-nightly-0.3.1", "--repo=evil/x", 1)
		},
		"a relative asset path": func(*PublishParams) string {
			return strings.Replace(string(good), `"path": "/`, `"path": "-x/`, 1)
		},
		"a destination that is an option": func(p *PublishParams) string { p.MetadataDest = "-e"; return string(good) },
		"a repository that is an option":  func(p *PublishParams) string { p.GiteaRepo = "--help"; return string(good) },
		"an API base that is not a URL":   func(p *PublishParams) string { p.GiteaAPI = "git.example.org/api/v1"; return string(good) },
		"no token":                        func(p *PublishParams) string { p.GiteaToken = ""; return string(good) },
	}
	for name, change := range cases {
		g, rec := newFakeGitea(t), &recorder{}
		p := publishParams(repo, rec, g)
		body := change(&p)
		if err := os.WriteFile(filepath.Join(repo.Dir, PendingFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Publish(t.Context(), p); err == nil {
			t.Errorf("%s was published", name)
		}
		if len(rec.calls) != 0 || g.requestCount() != 0 {
			t.Errorf("%s: something ran: rsync %q, %d requests", name, rec.calls, g.requestCount())
		}
	}
}

func TestPublish_noTokenSaysWhichVariableToSet(t *testing.T) {
	agent := newFakeAgent(t)
	repo := newRepo(t, agent)
	cutOne(t, repo, agent, "nightly", "0.3.1", "bytes")
	g := newFakeGitea(t)
	p := publishParams(repo, &recorder{}, g)
	p.GiteaToken = ""
	if err := Publish(t.Context(), p); err == nil || !strings.Contains(err.Error(), "GITEA_TOKEN") {
		t.Fatalf("err = %v", err)
	}
	p.DryRun = true
	if err := Publish(t.Context(), p); err != nil {
		t.Fatalf("a dry run needs no token: %v", err)
	}
}

func mustStat(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}
