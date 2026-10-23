package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
)

const corsOriginsPath = "/v1/namespace/cors-origins"

// NamespaceCORSOrigins shows a namespace's allowed browser origins, or replaces
// them when set is non-nil (an empty set clears the list).
func NamespaceCORSOrigins(out *printer.Printer, ns string, set []string) error {
	gatewayURL, token, err := loadAuthForNamespace(ns)
	if err != nil {
		return err
	}
	method, what := http.MethodGet, "read the allowed origins"
	var body io.Reader
	if set != nil {
		encoded, err := json.Marshal(map[string][]string{"origins": set})
		if err != nil {
			return clierr.Failure("encode the origins: %w", err)
		}
		method, what, body = http.MethodPut, "set the allowed origins", bytes.NewReader(encoded)
	}
	result, err := nsRequest(what, method, gatewayURL+corsOriginsPath, token, body)
	if err != nil {
		return err
	}
	if out.JSONMode() {
		return out.JSON(result)
	}
	origins, _ := result["origins"].([]any)
	fmt.Fprintf(out.Out(), "Allowed origins for namespace '%v'\n\n", result["namespace"])
	if len(origins) == 0 {
		fmt.Fprintf(out.Out(), "  none beyond the built-in rule (the base domain, localhost and the Tauri webviews)\n")
		return nil
	}
	for _, o := range origins {
		fmt.Fprintf(out.Out(), "  %v\n", o)
	}
	return nil
}
