//go:build e2e_fleet

package serverless

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Orama-Network/orama/e2e/harness/gw"
)

// versionRow matches one `orama function list --all` row: name, version, status.
func versionRow(name, version, status string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `\s+` + version + `\s+` + status + `\b`)
}

// mark invokes fn (name or name@N) as the admin and returns the MARK it ran with.
func mark(t *testing.T, fx *fixture, fn string) any {
	t.Helper()
	return call(t, fx, fn, map[string]any{"op": "whoami", "key": "MARK"})["env"]
}

// expectSuperseded: invoking fn answers 404 FUNCTION_VERSION_SUPERSEDED.
func expectSuperseded(t *testing.T, r *gw.Response, what string) {
	t.Helper()
	if r.Status != http.StatusNotFound || rpcCode(r) != codeSuperseded {
		t.Errorf("%s: want 404 %s, got %d %s %.200s", what, codeSuperseded, r.Status, rpcCode(r), r.Body)
	}
}

// TestVersions_onlyTheCurrentVersionRuns: after a second deploy name@1 is
// refused as superseded on /v1/functions/…/invoke and on /v1/invoke, a
// version that never existed is NOT_FOUND, a rollback makes version 1 the one
// that runs (bare name and @1) and supersedes version 2, and the next deploy
// is current again with the next number; `list --all` shows every row with
// its status (website/src/docs/developer/functions.mdx#versioning, bug-939, bug-940).
func TestVersions_onlyTheCurrentVersionRuns(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-ver"
	dir := deploy(t, fx, fnSpec{name: fn, yaml: "env:\n  MARK: v1\n"})
	redeploy(t, fx, dir, "env:\n  MARK: v2\n")

	for name, want := range map[string]string{fn: "v2", fn + "@2": "v2"} {
		if got := mark(t, fx, name); got != want {
			t.Errorf("%s ran MARK=%v, want %s", name, got, want)
		}
	}
	expectSuperseded(t, invoke(t, fx.c, fn+"@1", fx.admin, map[string]any{"op": "echo"}), fn+"@1 after v2 shipped")
	direct := fx.c.MustSend(t, gw.Req{Method: http.MethodPost, Path: "/v1/invoke/" + fx.n.Name + "/" + fn + "@1", Bearer: fx.admin,
		Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"op":"echo"}`)})
	expectSuperseded(t, direct, "POST /v1/invoke/<ns>/"+fn+"@1")
	if r := invoke(t, fx.c, fn+"@99", fx.admin, map[string]any{}); r.Status != http.StatusNotFound || rpcCode(r) != codeNotFound {
		t.Errorf("a version that does not exist: want 404 %s, got %d %s", codeNotFound, r.Status, rpcCode(r))
	}

	fx.n.CLI.MustOK(t, "function", "rollback", fn, "1")
	for name, want := range map[string]string{fn: "v1", fn + "@1": "v1"} {
		if got := mark(t, fx, name); got != want {
			t.Errorf("after rollback to 1, %s ran MARK=%v, want %s", name, got, want)
		}
	}
	expectSuperseded(t, invoke(t, fx.c, fn+"@2", fx.admin, map[string]any{"op": "echo"}), fn+"@2 after rolling back from it")
	if res := runCLI(t, fx, "function", "rollback", fn, "99"); res.Exit != 4 {
		t.Errorf("rollback to a version that does not exist: exit %d, want 4 (not found)\n%s%s", res.Exit, res.Stdout, res.Stderr)
	}

	redeploy(t, fx, dir, "env:\n  MARK: v3\n")
	if got := mark(t, fx, fn); got != "v3" {
		t.Errorf("deploy after rollback ran MARK=%v, want v3", got)
	}
	expectSuperseded(t, invoke(t, fx.c, fn+"@1", fx.admin, map[string]any{"op": "echo"}), fn+"@1 after the next deploy")
	all := fx.n.CLI.MustOK(t, "function", "list", "--all").Stdout
	for _, row := range []*regexp.Regexp{
		versionRow(fn, "3", "active"), versionRow(fn, "2", "superseded"), versionRow(fn, "1", "superseded"),
	} {
		if !row.MatchString(all) {
			t.Errorf("list --all has no row %s:\n%s", row, all)
		}
	}
	if out := fx.n.CLI.MustOK(t, "function", "list").Stdout; strings.Count(out, fn) != 1 {
		t.Errorf("list must show the function once, at its current version:\n%s", out)
	}
}

// TestVersions_deleteAndPurge: deleting one version marks it deleted and
// `list --all` still shows it; a purge is refused to a developer (who may
// delete) and to a runtime member, and the owner's purge removes every row,
// trigger and log of the function, after which it is not found anywhere and a
// new deploy starts at version 1 (website/src/docs/developer/functions.mdx#versioning, bug-940).
func TestVersions_deleteAndPurge(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-purge"
	dir := deploy(t, fx, fnSpec{name: fn, yaml: "env:\n  MARK: v1\n"})
	redeploy(t, fx, dir, "env:\n  MARK: v2\n")
	fx.n.CLI.MustOK(t, "function", "triggers", "add", fn, "--topic", "e2e:purge")
	call(t, fx, fn, map[string]any{"op": "echo", "value": "before-purge"})

	fx.n.CLI.MustOK(t, "function", "delete", fn, "--version", "1", "--force")
	all := fx.n.CLI.MustOK(t, "function", "list", "--all").Stdout
	if !versionRow(fn, "1", "deleted").MatchString(all) || !versionRow(fn, "2", "active").MatchString(all) {
		t.Errorf("list --all after deleting version 1:\n%s", all)
	}
	if res := runCLI(t, fx, "function", "rollback", fn, "1"); res.Exit != 4 {
		t.Errorf("rollback to a deleted version: exit %d, want 4", res.Exit)
	}

	developer := member(t, fx.n, fx.c, roleDeveloper)
	for who, bearer := range map[string]string{"developer": developer, "runtime": fx.runtime} {
		r := fx.c.MustSend(t, gw.Req{Method: http.MethodDelete, Path: "/v1/functions/" + fn + "?purge=true", Bearer: bearer})
		if r.Status != http.StatusForbidden {
			t.Errorf("purge by a %s member: want 403, got %d %.200s", who, r.Status, r.Body)
		}
	}
	if got := mark(t, fx, fn); got != "v2" {
		t.Fatalf("a refused purge changed the function: MARK=%v", got)
	}

	fx.n.CLI.MustOK(t, "function", "delete", fn, "--purge", "--force")
	if all := fx.n.CLI.MustOK(t, "function", "list", "--all").Stdout; strings.Contains(all, fn) {
		t.Errorf("list --all still shows %s after purge:\n%s", fn, all)
	}
	for _, args := range [][]string{
		{"function", "versions", fn},
		{"function", "triggers", "list", fn},
		{"function", "logs", fn},
		{"function", "invoke", fn},
	} {
		if res := runCLI(t, fx, args...); res.Exit != 4 {
			t.Errorf("%v after purge: exit %d, want 4 (not found)\n%s%s", args, res.Exit, res.Stdout, res.Stderr)
		}
	}
	if r := invoke(t, fx.c, fn+"@2", fx.admin, map[string]any{}); r.Status != http.StatusNotFound {
		t.Errorf("%s@2 after purge: want 404, got %d", fn, r.Status)
	}

	redeploy(t, fx, dir, "env:\n  MARK: fresh\n")
	if out := fx.n.CLI.MustOK(t, "function", "versions", fn).Stdout; !strings.Contains(out, "Total: 1") {
		t.Errorf("versions of a function deployed again after purge:\n%s", out)
	}
	if got := mark(t, fx, fn+"@1"); got != "fresh" {
		t.Errorf("%s@1 after purge and deploy ran MARK=%v, want fresh", fn, got)
	}
}

// TestVersions_wasmSHA256IsOfTheUploadedFile: deploying the same function.wasm
// twice records the same wasm_sha256 on both versions, equal to the file's
// SHA-256, in `function get`, `function versions` and the versions route,
// although the sealed upload's wasm_cid may differ
// (website/src/docs/developer/functions.mdx#versioning, chg-956).
func TestVersions_wasmSHA256IsOfTheUploadedFile(t *testing.T) {
	t.Parallel()
	fx := setup(t)
	const fn = "e2e-sha"
	dir := deploy(t, fx, fnSpec{name: fn})
	sum := sha256.Sum256(readFile(t, filepath.Join(dir, "function.wasm")))
	want := hex.EncodeToString(sum[:])
	redeploy(t, fx, dir, "")

	var got map[string]any
	if out := fx.n.CLI.MustOK(t, "function", "get", fn).Stdout; json.Unmarshal([]byte(out), &got) != nil || got["wasm_sha256"] != want {
		t.Errorf("function get: wasm_sha256 = %v, want %s (shasum of function.wasm)\n%s", got["wasm_sha256"], want, out)
	}
	if out := fx.n.CLI.MustOK(t, "function", "versions", fn).Stdout; strings.Count(out, want) != 2 {
		t.Errorf("function versions must show %s on both versions:\n%s", want, out)
	}

	r := fx.c.MustSend(t, gw.Req{Method: http.MethodGet, Path: "/v1/functions/" + fn + "/versions", Bearer: fx.admin})
	var listing struct {
		Versions []struct {
			Version    int     `json:"version"`
			WASMSHA256 *string `json:"wasm_sha256"`
			WASMCID    string  `json:"wasm_cid"`
		} `json:"versions"`
	}
	if r.Status != http.StatusOK || json.Unmarshal(r.Body, &listing) != nil || len(listing.Versions) != 2 {
		t.Fatalf("GET versions: %d %.300s", r.Status, r.Body)
	}
	for _, v := range listing.Versions {
		if v.WASMSHA256 == nil || *v.WASMSHA256 != want {
			t.Errorf("version %d: wasm_sha256 = %v, want %s", v.Version, v.WASMSHA256, want)
		}
	}
	t.Logf("wasm_cid of the two deploys: %s, %s", listing.Versions[0].WASMCID, listing.Versions[1].WASMCID)
}
