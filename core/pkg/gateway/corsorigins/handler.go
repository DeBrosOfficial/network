package corsorigins

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Orama-Network/orama/core/pkg/gateway/auth"
	"github.com/Orama-Network/orama/core/pkg/gateway/ctxkeys"
	"github.com/Orama-Network/orama/core/pkg/httputil"
	"go.uber.org/zap"
)

// maxBody bounds a PUT: MaxOriginsPerNamespace origins of maxOriginLength.
const maxBody = 16 * 1024

// Handler serves /v1/namespace/cors-origins: GET reads the caller's
// namespace's list, PUT {"origins": [...]} replaces it (an empty list clears
// it). The route policy requires namespace:write, which an owner and an admin
// hold and a developer does not, and pins the route to the index gateway, whose
// cache is the one the namespace's host is answered from.
type Handler struct {
	store  *Store
	cache  *Cache
	logger *zap.Logger
}

// NewHandler builds the handler over the store and the cache it updates.
func NewHandler(store *Store, cache *Cache, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{store: store, cache: cache, logger: logger}
}

type putRequest struct {
	Origins []string `json:"origins"`
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	namespace, _ := r.Context().Value(ctxkeys.NamespaceOverride).(string)
	namespace = normalizedNamespace(namespace)
	if namespace == "" {
		httputil.WriteRPCError(w, http.StatusUnauthorized, httputil.ErrCodeUnauthorized,
			"the namespace could not be resolved from this credential")
		return
	}
	if auth.IsLobbyNamespace(namespace) {
		httputil.WriteRPCError(w, http.StatusForbidden, httputil.ErrCodeForbidden,
			"the lobby serves no app; set allowed origins on a namespace you own")
		return
	}
	switch r.Method {
	case http.MethodGet:
		origins, err := h.store.Origins(r.Context(), namespace)
		if err != nil {
			h.unavailable(w, namespace, err)
			return
		}
		h.answer(w, namespace, origins)
	case http.MethodPut:
		h.put(w, r, namespace)
	default:
		w.Header().Set("Allow", "GET, PUT")
		httputil.WriteRPCError(w, http.StatusMethodNotAllowed, httputil.ErrCodeValidationFailed,
			`GET to read, PUT {"origins": [...]} to replace`)
	}
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request, namespace string) {
	var req putRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httputil.WriteRPCError(w, http.StatusBadRequest, httputil.ErrCodeValidationFailed,
			`the body is {"origins": ["https://app.example.com", ...]}`)
		return
	}
	origins, err := Normalize(req.Origins)
	if err != nil {
		httputil.WriteRPCError(w, http.StatusBadRequest, httputil.ErrCodeValidationFailed, err.Error())
		return
	}
	if err := h.store.Set(r.Context(), namespace, origins, auth.ActorFromRequest(r)); err != nil {
		if errors.Is(err, ErrNamespaceNotFound) {
			httputil.WriteRPCError(w, http.StatusNotFound, httputil.ErrCodeNotFound, "no such namespace")
			return
		}
		h.unavailable(w, namespace, err)
		return
	}
	h.cache.Put(namespace, origins)
	h.answer(w, namespace, origins)
}

func (h *Handler) answer(w http.ResponseWriter, namespace string, origins []string) {
	if origins == nil {
		origins = []string{}
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"namespace": namespace, "origins": origins})
}

// unavailable answers 503 without the registry's own words, which name hosts.
func (h *Handler) unavailable(w http.ResponseWriter, namespace string, err error) {
	h.logger.Warn("allowed origins unavailable", zap.String("namespace", namespace), zap.Error(err))
	httputil.WriteRPCError(w, http.StatusServiceUnavailable, httputil.ErrCodeServiceUnavailable,
		"the allowed origins could not be read or written right now; retry shortly", httputil.WithRetryable())
}

// NamespaceFromHost returns <name> for a host ns-<name>.<baseDomain>, with
// or without a port, and "" for any other host.
func NamespaceFromHost(host, baseDomain string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.LastIndex(host, ":"); i != -1 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	suffix := "." + strings.ToLower(strings.TrimSpace(baseDomain))
	if baseDomain == "" || !strings.HasSuffix(host, suffix) {
		return ""
	}
	sub := strings.TrimSuffix(host, suffix)
	if !strings.HasPrefix(sub, "ns-") || strings.Contains(sub, ".") {
		return ""
	}
	return strings.TrimPrefix(sub, "ns-")
}
