package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Orama-Network/orama/core/pkg/client"
)

// Which pub/sub topics a namespace's end users may read (bugboard #733).
//
// An end user is a wallet that signed in to an open namespace and holds no
// grant in it (NoGrantPermissions). It used to read every pub/sub topic of the
// namespace: subscribe to any of them, list them and see who was present on
// them. An application's topics are not all for its users — a chat app's
// server publishes every new message's sender and conversation on a topic its
// functions consume — and the application had no way to say which were. The
// namespace now lists them, and the list starts empty: an end user reads no
// topic until the owner names it.
//
// A pattern is a permission resource, so `*` matches any run of characters,
// `/` included, exactly as in a `pubsub:topic=` selector on a grant. A member
// holding a grant is not affected: their grant says what they read.

// MaxEndUserTopics bounds the list. It is read on every pub/sub request an end
// user makes, matched pattern by pattern.
const MaxEndUserTopics = 64

// reservedTopicPrefix is the prefix of the topics the platform publishes on.
// A grantless session never subscribes to them whatever the list says
// (handlers/pubsub refuseReservedSubscribe), so a pattern under it would grant
// nothing.
const reservedTopicPrefix = "_orama/"

// ParseEndUserTopics validates and canonicalizes a topic list: trimmed,
// deduplicated and sorted. An empty list is valid and means no topic.
func ParseEndUserTopics(patterns []string) ([]string, error) {
	seen := make(map[string]struct{}, len(patterns))
	out := make([]string, 0, len(patterns))
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" {
			return nil, fmt.Errorf("an end-user topic pattern cannot be empty")
		}
		if len(p) > maxSelectorLength {
			return nil, fmt.Errorf("an end-user topic pattern is at most %d characters", maxSelectorLength)
		}
		for _, c := range p {
			if c < 0x21 || c > 0x7e {
				return nil, fmt.Errorf("end-user topic pattern %q: only printable characters and no spaces", p)
			}
		}
		if len(p) >= len(reservedTopicPrefix) && strings.EqualFold(p[:len(reservedTopicPrefix)], reservedTopicPrefix) {
			return nil, fmt.Errorf("end-user topic pattern %q: topics under %q are the platform's and no end user reads them", p, reservedTopicPrefix)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) > MaxEndUserTopics {
		return nil, fmt.Errorf("at most %d end-user topic patterns (got %d)", MaxEndUserTopics, len(out))
	}
	sort.Strings(out)
	return out, nil
}

// EndUserTopicsOf returns the topics a namespace's end users may read, as
// recorded now. A namespace with no row, and the lobby, has none.
func (s *Service) EndUserTopicsOf(ctx context.Context, namespace string) ([]string, error) {
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return []string{}, nil
	}
	db, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return nil, err
	}
	res, err := db.Query(client.WithInternalAuth(ctx),
		`SELECT end_user_topics FROM namespace_session_policy WHERE namespace_id = ? LIMIT 1`, nsID)
	if err != nil {
		return nil, fmt.Errorf("read the end-user topics of %q: %w", namespace, err)
	}
	topics := []string{}
	if res != nil && len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
		if err := json.Unmarshal([]byte(getStringVal(res.Rows[0][0])), &topics); err != nil {
			return nil, fmt.Errorf("the end-user topics of %q are not a JSON list: %w", namespace, err)
		}
	}
	s.endUserTopics.put(namespace, topics, time.Now())
	return topics, nil
}

// CachedEndUserTopicsOf is EndUserTopicsOf, answered from what this gateway
// read within devicePolicyStaleness: it is asked on every pub/sub read an end
// user makes, so a change the owner makes reaches every gateway within that
// bound instead of costing a registry read per request.
func (s *Service) CachedEndUserTopicsOf(ctx context.Context, namespace string) ([]string, error) {
	namespace = sessionNamespace(namespace)
	if topics, ok := s.endUserTopics.get(namespace, time.Now()); ok {
		return topics, nil
	}
	return s.EndUserTopicsOf(ctx, namespace)
}

// SetEndUserTopics records the topics a namespace's end users may read,
// replacing the list. It writes only that column: a row it creates starts with
// the device and sign-in policies a namespace with no row has.
func (s *Service) SetEndUserTopics(ctx context.Context, namespace string, patterns []string, actor string) ([]string, error) {
	topics, err := ParseEndUserTopics(patterns)
	if err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, ErrRotationNotConfigured
	}
	namespace = sessionNamespace(namespace)
	if IsLobbyNamespace(namespace) {
		return nil, ErrLobbyHasNoSignInPolicy
	}
	_, nsID, err := s.deviceRegistry(ctx, namespace)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(topics)
	if err != nil {
		return nil, fmt.Errorf("encode the end-user topics: %w", err)
	}
	if _, err := s.db.Exec(client.WithInternalAuth(ctx),
		`INSERT INTO namespace_session_policy(namespace_id, device_policy, sign_in, end_user_topics, updated_by, updated_at)
		 VALUES (?, ?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(namespace_id) DO UPDATE SET
		   end_user_topics = excluded.end_user_topics,
		   updated_by = excluded.updated_by,
		   updated_at = excluded.updated_at`,
		nsID, string(DevicePolicyOptional), string(SignInMembers), string(encoded), actor); err != nil {
		return nil, fmt.Errorf("record the end-user topics of %q: %w", namespace, err)
	}
	s.endUserTopics.put(namespace, topics, time.Now())
	return topics, nil
}
