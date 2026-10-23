// Package notify pushes the verdict of a fleet run to the owner's phone
// through ntfy (https://docs.ntfy.sh/publish/): one POST of the message body
// to the topic URL, with the title, priority and tags as headers.
//
// The topic URL is NTFY_URL from Infisical (orama-e2e, folder /notify), the
// Orama ntfy at push.<base domain> and a topic only the owner subscribes to.
// That server has no auth file, so the topic name is the credential: the URL
// is a run secret (secrets.SecretEnvNames), sealed and redacted like the
// others, and never printed.
package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EnvURL is the ntfy topic URL.
const EnvURL = "NTFY_URL"

// requestTimeout bounds the one publish request.
const requestTimeout = 30 * time.Second

// maxErrorBody is how much of a refused publish's answer an error quotes.
const maxErrorBody = 512

// ntfy priorities (https://docs.ntfy.sh/publish/#message-priority).
const (
	PriorityDefault = 3
	PriorityHigh    = 4
)

// Message is one push.
type Message struct {
	Title    string
	Body     string
	Priority int
	// Tags are ntfy tags; a tag that names an emoji shows as that emoji.
	Tags []string
}

// Ntfy publishes to one topic.
type Ntfy struct {
	topic string
	http  *http.Client
}

// New checks the topic URL: an https URL with a topic path. It never puts the
// URL in an error, since the topic is the credential.
func New(topic string) (*Ntfy, error) {
	u, err := url.Parse(topic)
	if err != nil || u.Scheme != "https" || u.Host == "" || strings.Trim(u.Path, "/") == "" || strings.Contains(strings.Trim(u.Path, "/"), "/") {
		return nil, fmt.Errorf("%s is not an https ntfy topic URL (https://push.<base domain>/<topic>)", EnvURL)
	}
	return &Ntfy{topic: topic, http: &http.Client{Timeout: requestTimeout}}, nil
}

// Send publishes m and fails unless ntfy accepted it.
func (n *Ntfy) Send(ctx context.Context, m Message) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.topic, strings.NewReader(m.Body))
	if err != nil {
		return fmt.Errorf("build the ntfy request: %w", err)
	}
	req.Header.Set("Title", m.Title)
	req.Header.Set("Priority", fmt.Sprint(m.Priority))
	if len(m.Tags) > 0 {
		req.Header.Set("Tags", strings.Join(m.Tags, ","))
	}
	res, err := n.http.Do(req)
	if err != nil {
		// The error names the URL; report the failure without it.
		return fmt.Errorf("push to ntfy failed: %w", unwrapURL(err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, maxErrorBody))
		return fmt.Errorf("ntfy refused the push: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// unwrapURL drops the request URL from a transport error.
func unwrapURL(err error) error {
	if ue, ok := err.(*url.Error); ok {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}
