package releasepub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	// assetFormField is the multipart field Gitea reads a release asset from.
	assetFormField = "attachment"
	// maxResponseBody bounds what is read back from the Gitea API.
	maxResponseBody = 1 << 20
	// maxErrorBody is how much of a failed response goes into the error.
	maxErrorBody = 512
)

// giteaAsset is one asset of a release as the Gitea API lists it. Gitea
// reports no digest, so the size is all there is to compare.
type giteaAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// giteaRelease is the part of a Gitea release that publish reads.
type giteaRelease struct {
	ID     int64        `json:"id"`
	Assets []giteaAsset `json:"assets"`
}

// uploadAssets creates the Gitea release for the tag if there is none and
// uploads the archives it does not hold yet. An archive already there is
// skipped when it has the size of the one the cut listed, so a publish that
// stopped after the upload can be run again; one that differs is an error,
// because a published archive is never overwritten.
func (p PublishParams) uploadAssets(ctx context.Context, pending *Pending) error {
	if p.DryRun {
		p.planUpload(pending)
		return nil
	}
	fmt.Fprintf(p.Progress, "Uploading %d archive(s) to %s release %s...\n", len(pending.Assets), p.GiteaRepo, pending.Tag)
	release, err := p.releaseFor(ctx, pending)
	if err != nil {
		return err
	}
	held := make(map[string]giteaAsset, len(release.Assets))
	for _, a := range release.Assets {
		held[a.Name] = a
	}
	var missing []Asset
	for _, a := range pending.Assets {
		have, ok := held[a.Name]
		if !ok {
			missing = append(missing, a)
			continue
		}
		if err := sameAsset(a, have); err != nil {
			return err
		}
	}
	for _, a := range missing {
		if err := p.uploadAsset(ctx, release.ID, pending.Tag, a); err != nil {
			return err
		}
	}
	return nil
}

// planUpload prints what a real run would do. It carries no credentials.
func (p PublishParams) planUpload(pending *Pending) {
	fmt.Fprintf(p.Progress, "Would upload %d archive(s) to %s release %s:\n", len(pending.Assets), p.GiteaRepo, pending.Tag)
	p.print("GET", []string{p.tagURL(pending.Tag)}, "(create the release if it is not there, and upload the archives it lacks:)")
	for _, a := range pending.Assets {
		p.print("upload", []string{a.Path, "->", pending.Tag, "asset", a.Name}, "")
	}
}

// releaseFor is the release of the tag, created if there is none.
func (p PublishParams) releaseFor(ctx context.Context, pending *Pending) (*giteaRelease, error) {
	status, body, err := p.call(ctx, http.MethodGet, p.tagURL(pending.Tag), "", nil, -1)
	if err != nil {
		return nil, fmt.Errorf("look up the Gitea release for %s: %w", pending.Tag, err)
	}
	switch status {
	case http.StatusOK:
		return parseRelease(body, pending.Tag)
	case http.StatusNotFound:
		return p.createRelease(ctx, pending)
	default:
		return nil, statusError("look up the Gitea release for "+pending.Tag, status, body)
	}
}

func (p PublishParams) createRelease(ctx context.Context, pending *Pending) (*giteaRelease, error) {
	doc, err := json.Marshal(map[string]any{
		"tag_name":   pending.Tag,
		"name":       pending.Channel + " " + pending.Version,
		"body":       fmt.Sprintf("Orama %s %s. Install it through the TUF metadata at the release host; this page is only where the bytes live.", pending.Channel, pending.Version),
		"draft":      false,
		"prerelease": pending.Channel != ChannelMain,
	})
	if err != nil {
		return nil, fmt.Errorf("encode the Gitea release for %s: %w", pending.Tag, err)
	}
	status, body, err := p.call(ctx, http.MethodPost, p.repoURL("/releases"), "application/json", bytes.NewReader(doc), int64(len(doc)))
	if err != nil {
		return nil, fmt.Errorf("create the Gitea release for %s: %w", pending.Tag, err)
	}
	if status != http.StatusCreated {
		return nil, statusError("create the Gitea release for "+pending.Tag, status, body)
	}
	return parseRelease(body, pending.Tag)
}

// uploadAsset sends one archive as a multipart upload, streamed from disk.
func (p PublishParams) uploadAsset(ctx context.Context, releaseID int64, tag string, a Asset) error {
	f, err := os.Open(a.Path)
	if err != nil {
		return fmt.Errorf("open the archive %s: %w", a.Path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", a.Path, err)
	}
	// The multipart framing around the file is built up front so the request
	// has a known length instead of being chunked.
	var framing bytes.Buffer
	mw := multipart.NewWriter(&framing)
	if _, err := mw.CreateFormFile(assetFormField, a.Name); err != nil {
		return fmt.Errorf("frame the upload of %s: %w", a.Name, err)
	}
	headLen := framing.Len()
	if err := mw.Close(); err != nil {
		return fmt.Errorf("frame the upload of %s: %w", a.Name, err)
	}
	head, tail := framing.Bytes()[:headLen], framing.Bytes()[headLen:]
	body := io.MultiReader(bytes.NewReader(head), f, bytes.NewReader(tail))
	length := int64(len(head)) + info.Size() + int64(len(tail))

	fmt.Fprintf(p.Progress, "Uploading %s...\n", a.Name)
	endpoint := fmt.Sprintf("%s?name=%s", p.repoURL(fmt.Sprintf("/releases/%d/assets", releaseID)), url.QueryEscape(a.Name))
	status, resp, err := p.call(ctx, http.MethodPost, endpoint, mw.FormDataContentType(), body, length)
	if err != nil {
		return fmt.Errorf("upload %s to %s: %w", a.Name, tag, err)
	}
	if status != http.StatusCreated {
		return statusError(fmt.Sprintf("upload %s to %s", a.Name, tag), status, resp)
	}
	return nil
}

// call sends one authenticated request and returns the status and the body.
// length is the request body's size, or -1 when there is none.
func (p PublishParams) call(ctx context.Context, method, target, contentType string, body io.Reader, length int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, fmt.Errorf("build the %s request: %w", method, err)
	}
	if length >= 0 {
		req.ContentLength = length
	}
	req.Header.Set("Authorization", "token "+p.GiteaToken)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		// A *url.Error carries the URL, never the headers.
		return 0, nil, fmt.Errorf("%s %s: %w", method, p.GiteaAPI, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return 0, nil, fmt.Errorf("read the response of %s %s: %w", method, p.GiteaAPI, err)
	}
	return resp.StatusCode, data, nil
}

// repoURL is a path under the repository's API root.
func (p PublishParams) repoURL(suffix string) string {
	segments := strings.Split(p.GiteaRepo, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.TrimRight(p.GiteaAPI, "/") + "/repos/" + strings.Join(segments, "/") + suffix
}

func (p PublishParams) tagURL(tag string) string {
	return p.repoURL("/releases/tags/" + url.PathEscape(tag))
}

func parseRelease(body []byte, tag string) (*giteaRelease, error) {
	var r giteaRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("read the Gitea release for %s: %w", tag, err)
	}
	if r.ID == 0 {
		return nil, fmt.Errorf("the Gitea release for %s has no id", tag)
	}
	return &r, nil
}

func statusError(what string, status int, body []byte) error {
	text := strings.TrimSpace(string(body))
	if len(text) > maxErrorBody {
		text = text[:maxErrorBody] + "..."
	}
	return fmt.Errorf("%s: Gitea answered HTTP %d: %s (check GITEA_TOKEN and --gitea-repo)", what, status, text)
}

// sameAsset refuses a published asset that is not the archive the cut listed.
func sameAsset(want Asset, have giteaAsset) error {
	info, err := os.Stat(want.Path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", want.Path, err)
	}
	if have.Size != info.Size() {
		return fmt.Errorf("the Gitea release already holds %s with %d bytes, not the cut's %d: a published archive is never overwritten (cut a new version)", want.Name, have.Size, info.Size())
	}
	return nil
}
