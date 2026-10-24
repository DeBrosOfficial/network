// Command function is the auth-claims-provider the webrtc feature deploys with
// `orama function deploy`: TinyGo, WASI, stateless. At sign-in the gateway hands
// it {"wallet","namespace"} and signs the claims it prints into the token. It
// names every wallet's account "acct-" + the first 16 hex digits of the SHA-256
// of the lowercased wallet, which the test derives the same way, so the account
// carries nothing a peer could read the wallet from.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
)

const (
	accountPrefix    = "acct-"
	accountHexDigits = 16
)

func main() {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}
	var req struct {
		Wallet string `json:"wallet"`
	}
	if json.Unmarshal(in, &req) != nil || req.Wallet == "" {
		return
	}
	sum := sha256.Sum256([]byte(strings.ToLower(req.Wallet)))
	out, err := json.Marshal(map[string]string{"account_id": accountPrefix + hex.EncodeToString(sum[:])[:accountHexDigits]})
	if err != nil {
		return
	}
	os.Stdout.Write(out)
}
