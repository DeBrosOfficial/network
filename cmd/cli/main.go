package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"git.debros.io/DeBros/network/pkg/anyoneproxy"
	"git.debros.io/DeBros/network/pkg/auth"
	"git.debros.io/DeBros/network/pkg/client"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	bootstrapPeer = "/ip4/127.0.0.1/tcp/4001"
	timeout       = 30 * time.Second
	format        = "table"
	useProduction = false
	disableAnon   = false
)

// version metadata populated via -ldflags at build time
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	// Parse global flags
	parseGlobalFlags(args)

	// Apply disable flag early so all network operations honor it
	anyoneproxy.SetDisabled(disableAnon)

	switch command {
	case "version":
		fmt.Printf("network-cli %s", version)
		if commit != "" {
			fmt.Printf(" (commit %s)", commit)
		}
		if date != "" {
			fmt.Printf(" built %s", date)
		}
		fmt.Println()
		return
	case "health":
		handleHealth()
	case "peers":
		handlePeers()
	case "status":
		handleStatus()
	case "query":
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli query <sql>\n")
			os.Exit(1)
		}
		handleQuery(args[0])
	case "storage":
		handleStorage(args)
	case "pubsub":
		handlePubSub(args)
	case "connect":
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli connect <peer_address>\n")
			os.Exit(1)
		}
		handleConnect(args[0])
	case "peer-id":
		handlePeerID()
	case "help", "--help", "-h":
		showHelp()
	case "auth":
		handleAuth(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showHelp()
		os.Exit(1)
	}
}

func parseGlobalFlags(args []string) {
	for i, arg := range args {
		switch arg {
		case "-b", "--bootstrap":
			if i+1 < len(args) {
				bootstrapPeer = args[i+1]
			}
		case "-f", "--format":
			if i+1 < len(args) {
				format = args[i+1]
			}
		case "-t", "--timeout":
			if i+1 < len(args) {
				if d, err := time.ParseDuration(args[i+1]); err == nil {
					timeout = d
				}
			}
		case "--production":
			useProduction = true
		case "--disable-anonrc":
			disableAnon = true
		}
	}
}

func handleHealth() {
	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	health, err := client.Health()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get health: %v\n", err)
		os.Exit(1)
	}

	if format == "json" {
		printJSON(health)
	} else {
		printHealth(health)
	}
}

func handlePeers() {
	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	peers, err := client.Network().GetPeers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get peers: %v\n", err)
		os.Exit(1)
	}

	if format == "json" {
		printJSON(peers)
	} else {
		printPeers(peers)
	}
}

func handleStatus() {
	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	status, err := client.Network().GetStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
		os.Exit(1)
	}

	if format == "json" {
		printJSON(status)
	} else {
		printStatus(status)
	}
}

func handleQuery(sql string) {
	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := client.Database().Query(ctx, sql)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute query: %v\n", err)
		os.Exit(1)
	}

	if format == "json" {
		printJSON(result)
	} else {
		printQueryResult(result)
	}
}

func handleStorage(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: network-cli storage <get|put|list> [args...]\n")
		os.Exit(1)
	}

	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	subcommand := args[0]
	switch subcommand {
	case "get":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli storage get <key>\n")
			os.Exit(1)
		}
		value, err := client.Storage().Get(ctx, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get value: %v\n", err)
			os.Exit(1)
		}

		// Try to decode if it looks like base64
		decoded := tryDecodeBase64(string(value))
		fmt.Printf("%s\n", decoded)

	case "put":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli storage put <key> <value>\n")
			os.Exit(1)
		}
		err := client.Storage().Put(ctx, args[1], []byte(args[2]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to store value: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Stored key: %s\n", args[1])

	case "list":
		prefix := ""
		if len(args) > 1 {
			prefix = args[1]
		}
		keys, err := client.Storage().List(ctx, prefix, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list keys: %v\n", err)
			os.Exit(1)
		}
		if format == "json" {
			printJSON(keys)
		} else {
			for _, key := range keys {
				fmt.Println(key)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown storage command: %s\n", subcommand)
		os.Exit(1)
	}
}

func handlePubSub(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: network-cli pubsub <publish|subscribe|topics> [args...]\n")
		os.Exit(1)
	}

	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	subcommand := args[0]
	switch subcommand {
	case "publish":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli pubsub publish <topic> <message>\n")
			os.Exit(1)
		}
		err := client.PubSub().Publish(ctx, args[1], []byte(args[2]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to publish message: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Published message to topic: %s\n", args[1])

	case "subscribe":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: network-cli pubsub subscribe <topic> [duration]\n")
			os.Exit(1)
		}
		duration := 30 * time.Second
		if len(args) > 2 {
			if d, err := time.ParseDuration(args[2]); err == nil {
				duration = d
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		fmt.Printf("🔔 Subscribing to topic '%s' for %v...\n", args[1], duration)

		messageHandler := func(topic string, data []byte) error {
			fmt.Printf("📨 [%s] %s: %s\n", time.Now().Format("15:04:05"), topic, string(data))
			return nil
		}

		err := client.PubSub().Subscribe(ctx, args[1], messageHandler)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to subscribe: %v\n", err)
			os.Exit(1)
		}

		<-ctx.Done()
		fmt.Printf("✅ Subscription ended\n")

	case "topics":
		topics, err := client.PubSub().ListTopics(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list topics: %v\n", err)
			os.Exit(1)
		}
		if format == "json" {
			printJSON(topics)
		} else {
			for _, topic := range topics {
				fmt.Println(topic)
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown pubsub command: %s\n", subcommand)
		os.Exit(1)
	}
}

// handleAuth launches a local webpage to perform wallet signature and obtain an API key.
// Usage: network-cli auth [--gateway <url>] [--namespace <ns>] [--wallet <evm_addr>] [--plan <free|premium>]
func handleAuth(args []string) {
	// Defaults
	gatewayURL := getenvDefault("GATEWAY_URL", "http://localhost:8080")
	namespace := getenvDefault("GATEWAY_NAMESPACE", "default")
	wallet := ""
	plan := "free"

	// Parse simple flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--gateway":
			if i+1 < len(args) {
				gatewayURL = strings.TrimSpace(args[i+1])
				i++
			}
		case "--namespace":
			if i+1 < len(args) {
				namespace = strings.TrimSpace(args[i+1])
				i++
			}
		case "--wallet":
			if i+1 < len(args) {
				wallet = strings.TrimSpace(args[i+1])
				i++
			}
		case "--plan":
			if i+1 < len(args) {
				plan = strings.TrimSpace(strings.ToLower(args[i+1]))
				i++
			}
		}
	}

	// Spin up local HTTP server on random port
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to listen: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	// Normalize URL host to localhost for consistency with gateway default
	parts := strings.Split(addr, ":")
	listenURL := "http://localhost:" + parts[len(parts)-1] + "/"

	// Channel to receive API key
	type result struct {
		APIKey    string `json:"api_key"`
		Namespace string `json:"namespace"`
	}
	resCh := make(chan result, 1)
	srv := &http.Server{}

	mux := http.NewServeMux()
	// Root serves the HTML page with embedded gateway URL and defaults
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html>
<html>
<head><meta charset="utf-8"><title>DeBros Auth</title>
<style>body{font-family:system-ui,-apple-system,Segoe UI,Roboto,Arial,sans-serif;margin:2rem;max-width:720px}input,button,select{font-size:1rem;padding:.5rem;margin:.25rem 0}code{background:#f5f5f5;padding:.2rem .4rem;border-radius:4px}</style>
</head>
<body>
<h2>Authenticate with Wallet to Get API Key</h2>
<p>This will create or return an API key for namespace <code id="ns"></code> on gateway <code id="gw"></code>.</p>
<label>Wallet Address</label><br>
<input id="wallet" placeholder="0x..." style="width:100%%"/><br>
<label>Plan</label><br>
<select id="plan"><option value="free">free</option><option value="premium">premium (0.1 ETH)</option></select><br>
<button id="connect">Connect Wallet</button>
<button id="sign">Sign & Generate API Key</button>
<pre id="out" style="white-space:pre-wrap"></pre>
<script>
const GATEWAY = %q;
const DEFAULT_NS = %q;
const DEFAULT_WALLET = %q;
document.getElementById('gw').textContent = GATEWAY;
document.getElementById('ns').textContent = DEFAULT_NS;
document.getElementById('wallet').value = DEFAULT_WALLET;
document.getElementById('plan').value = %q;
const out = document.getElementById('out');
function log(m){ out.textContent += m + "\n" }
document.getElementById('connect').onclick = async () => {
  if (!window.ethereum) { log('No wallet provider found (window.ethereum). Install MetaMask.'); return }
  try { await window.ethereum.request({ method:'eth_requestAccounts' }); log('Wallet connected.'); } catch(e){ log('Connect failed: '+e.message) }
};
document.getElementById('sign').onclick = async () => {
  try {
    const wallet = document.getElementById('wallet').value.trim();
    const plan = document.getElementById('plan').value;
    if (!/^0x[0-9a-fA-F]{40}$/.test(wallet)) { log('Enter a valid EVM address'); return }
    // Request nonce
    const ch = await fetch(GATEWAY+"/v1/auth/challenge", {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({wallet, purpose:'api_key', namespace: DEFAULT_NS})});
    if (!ch.ok) { const t = await ch.text(); log('Challenge failed: '+t); return }
    const cj = await ch.json();
    const nonce = cj.nonce;
    // Sign nonce
    let sig = await window.ethereum.request({ method:'personal_sign', params:[ nonce, wallet ] });
    // Issue or fetch API key
    const resp = await fetch(GATEWAY+"/v1/auth/api-key", {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify({wallet, nonce, signature: sig, namespace: DEFAULT_NS, plan})});
    if (!resp.ok) { const t = await resp.text(); log('Issue API key failed: '+t); return }
    const data = await resp.json();
    log('API Key: '+data.api_key+'\nNamespace: '+data.namespace);
    // Send back to CLI
    await fetch('/callback', {method:'POST', headers:{'Content-Type':'application/json'}, body: JSON.stringify(data)});
  } catch(e){ log('Error: '+e.message) }
};
</script>
</body></html>`, gatewayURL, namespace, wallet, plan)
	})
	// Callback to deliver API key back to CLI
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			APIKey    string `json:"api_key"`
			Namespace string `json:"namespace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.APIKey) == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case resCh <- result{APIKey: payload.APIKey, Namespace: payload.Namespace}:
		default:
		}
		_, _ = w.Write([]byte("ok"))
		go func() { time.Sleep(500 * time.Millisecond); _ = srv.Close() }()
	})
	srv.Handler = mux

	// Open browser
	url := listenURL
	go func() {
		// Try to open in default browser
		_ = openBrowser(url)
	}()

	// Serve and wait for result or timeout
	go func() { _ = srv.Serve(ln) }()
	fmt.Printf("🌐 Please complete authentication in your browser: %s\n", url)
	select {
	case r := <-resCh:
		fmt.Printf("✅ API Key issued for namespace '%s'\n", r.Namespace)
		fmt.Printf("%s\n", r.APIKey)
	case <-time.After(5 * time.Minute):
		fmt.Fprintf(os.Stderr, "Timed out waiting for wallet signature.\n")
		_ = srv.Close()
		os.Exit(1)
	}
}

func openBrowser(target string) error {
	cmds := [][]string{
		{"xdg-open", target},
		{"open", target},
		{"cmd", "/c", "start", target},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	log.Printf("Please open %s manually", target)
	return nil
}

// getenvDefault returns env var or default if empty/undefined.
func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func handleConnect(peerAddr string) {
	client, err := createClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = client.Network().ConnectToPeer(ctx, peerAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to peer: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Connected to peer: %s\n", peerAddr)
}

func handlePeerID() {
	// Try to get peer ID from running network first
	client, err := createClient()
	if err == nil {
		defer client.Disconnect()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if status, err := client.Network().GetStatus(ctx); err == nil {
			if format == "json" {
				printJSON(map[string]string{"peer_id": status.NodeID})
			} else {
				fmt.Printf("🆔 Peer ID: %s\n", status.NodeID)
			}
			return
		}
	}

	// Fallback: try to extract from local identity files
	identityPaths := []string{
		"/opt/debros/data/node/identity.key",
		"/opt/debros/data/bootstrap/identity.key",
		"/opt/debros/keys/node/identity.key",
		"./data/node/identity.key",
		"./data/bootstrap/identity.key",
	}

	for _, path := range identityPaths {
		if peerID := extractPeerIDFromFile(path); peerID != "" {
			if format == "json" {
				printJSON(map[string]string{"peer_id": peerID, "source": "local_identity"})
			} else {
				fmt.Printf("🆔 Peer ID: %s\n", peerID)
				fmt.Printf("📂 Source: %s\n", path)
			}
			return
		}
	}

	// Check peer.info files as last resort
	peerInfoPaths := []string{
		"/opt/debros/data/node/peer.info",
		"/opt/debros/data/bootstrap/peer.info",
		"./data/node/peer.info",
		"./data/bootstrap/peer.info",
	}

	for _, path := range peerInfoPaths {
		if data, err := os.ReadFile(path); err == nil {
			multiaddr := strings.TrimSpace(string(data))
			if peerID := extractPeerIDFromMultiaddr(multiaddr); peerID != "" {
				if format == "json" {
					printJSON(map[string]string{"peer_id": peerID, "source": "peer_info"})
				} else {
					fmt.Printf("🆔 Peer ID: %s\n", peerID)
					fmt.Printf("📂 Source: %s\n", path)
				}
				return
			}
		}
	}

	fmt.Fprintf(os.Stderr, "❌ Could not find peer ID. Make sure the node is running or identity files exist.\n")
	os.Exit(1)
}

func createClient() (client.NetworkClient, error) {
	config := client.DefaultClientConfig("network-cli")

	// Check for existing credentials
	creds, err := auth.GetValidCredentials()
	if err != nil {
		// No valid credentials found, trigger authentication flow
		fmt.Printf("🔐 Authentication required for DeBros Network CLI\n")
		fmt.Printf("💡 This will open your browser to authenticate with your wallet\n")

		gatewayURL := auth.GetDefaultGatewayURL()
		fmt.Printf("🌐 Gateway: %s\n\n", gatewayURL)

		// Perform wallet authentication
		newCreds, authErr := auth.PerformWalletAuthentication(gatewayURL)
		if authErr != nil {
			return nil, fmt.Errorf("authentication failed: %w", authErr)
		}

		// Save credentials
		if saveErr := auth.SaveCredentialsForDefaultGateway(newCreds); saveErr != nil {
			fmt.Printf("⚠️  Warning: failed to save credentials: %v\n", saveErr)
		} else {
			fmt.Printf("💾 Credentials saved to ~/.debros/credentials.json\n")
		}

		creds = newCreds
	}

	// Configure client with API key
	config.APIKey = creds.APIKey

	// Update last used time
	creds.UpdateLastUsed()
	auth.SaveCredentialsForDefaultGateway(creds) // Best effort save

	networkClient, err := client.NewClient(config)
	if err != nil {
		return nil, err
	}

	if err := networkClient.Connect(); err != nil {
		return nil, err
	}

	return networkClient, nil
}

// discoverBootstrapPeer tries to find the bootstrap peer from saved peer info
func discoverBootstrapPeer() string {
	// Look for peer info in common locations
	peerInfoPaths := []string{
		"./data/bootstrap/peer.info",
		"./data/test-bootstrap/peer.info",
		"/tmp/bootstrap-peer.info",
	}

	for _, path := range peerInfoPaths {
		if data, err := os.ReadFile(path); err == nil {
			peerAddr := strings.TrimSpace(string(data))
			if peerAddr != "" {
				// Only print discovery message in table format
				if format != "json" {
					fmt.Printf("🔍 Discovered bootstrap peer: %s\n", peerAddr)
				}
				return peerAddr
			}
		}
	}

	return "" // Return empty string if no peer info found
}

// tryDecodeBase64 attempts to decode a string as base64, returns original if not valid base64
func tryDecodeBase64(s string) string {
	// Only try to decode if it looks like base64 (no spaces, reasonable length)
	if len(s) > 0 && len(s)%4 == 0 && !strings.ContainsAny(s, " \n\r\t") {
		if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
			// Check if decoded result looks like readable text
			decodedStr := string(decoded)
			if isPrintableText(decodedStr) {
				return decodedStr
			}
		}
	}
	return s
}

// isPrintableText checks if a string contains mostly printable characters
func isPrintableText(s string) bool {
	printableCount := 0
	for _, r := range s {
		if r >= 32 && r <= 126 || r == '\n' || r == '\r' || r == '\t' {
			printableCount++
		}
	}
	return len(s) > 0 && float64(printableCount)/float64(len(s)) > 0.8
}

func showHelp() {
	fmt.Printf("Network CLI - Distributed P2P Network Management Tool\n\n")
	fmt.Printf("Usage: network-cli <command> [args...]\n\n")
	fmt.Printf("Commands:\n")
	fmt.Printf("  health                    - Check network health\n")
	fmt.Printf("  peers                     - List connected peers\n")
	fmt.Printf("  status                    - Show network status\n")
	fmt.Printf("  peer-id                   - Show this node's peer ID\n")
	fmt.Printf("  query <sql>               - Execute database query\n")
	fmt.Printf("  storage get <key>         - Get value from storage\n")
	fmt.Printf("  storage put <key> <value> - Store value in storage\n")
	fmt.Printf("  storage list [prefix]     - List storage keys\n")
	fmt.Printf("  pubsub publish <topic> <msg> - Publish message\n")
	fmt.Printf("  pubsub subscribe <topic> [duration] - Subscribe to topic\n")
	fmt.Printf("  pubsub topics             - List topics\n")
	fmt.Printf("  connect <peer_address>    - Connect to peer\n")
	fmt.Printf("  auth [--gateway URL] [--namespace NS] [--wallet 0x..] [--plan free|premium] - Obtain API key via wallet signature\n")
	fmt.Printf("  help                      - Show this help\n\n")
	fmt.Printf("Global Flags:\n")
	fmt.Printf("  -b, --bootstrap <addr>    - Bootstrap peer address (default: /ip4/127.0.0.1/tcp/4001)\n")
	fmt.Printf("  -f, --format <format>     - Output format: table, json (default: table)\n")
	fmt.Printf("  -t, --timeout <duration>  - Operation timeout (default: 30s)\n")
	fmt.Printf("  --production              - Connect to production bootstrap peers\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  network-cli health\n")
	fmt.Printf("  network-cli peer-id\n")
	fmt.Printf("  network-cli peer-id --format json\n")
	fmt.Printf("  network-cli peers --format json\n")
	fmt.Printf("  network-cli peers --production\n")
	fmt.Printf("  network-cli storage put user:123 '{\"name\":\"Alice\"}'\n")
	fmt.Printf("  network-cli pubsub subscribe notifications 1m\n")
}

// Print functions

func printHealth(health *client.HealthStatus) {
	fmt.Printf("🏥 Network Health\n")
	fmt.Printf("Status: %s\n", getStatusEmoji(health.Status)+health.Status)
	fmt.Printf("Last Updated: %s\n", health.LastUpdated.Format("2006-01-02 15:04:05"))
	fmt.Printf("Response Time: %v\n", health.ResponseTime)
	fmt.Printf("\nChecks:\n")
	for check, status := range health.Checks {
		emoji := "✅"
		if status != "ok" {
			emoji = "❌"
		}
		fmt.Printf("  %s %s: %s\n", emoji, check, status)
	}
}

func printPeers(peers []client.PeerInfo) {
	fmt.Printf("👥 Connected Peers (%d)\n\n", len(peers))
	if len(peers) == 0 {
		fmt.Printf("No peers connected\n")
		return
	}

	for i, peer := range peers {
		connEmoji := "🔴"
		if peer.Connected {
			connEmoji = "🟢"
		}
		fmt.Printf("%d. %s %s\n", i+1, connEmoji, peer.ID)
		fmt.Printf("   Addresses: %v\n", peer.Addresses)
		fmt.Printf("   Last Seen: %s\n", peer.LastSeen.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}
}

func printStatus(status *client.NetworkStatus) {
	fmt.Printf("🌐 Network Status\n")
	fmt.Printf("Node ID: %s\n", status.NodeID)
	fmt.Printf("Connected: %s\n", getBoolEmoji(status.Connected)+strconv.FormatBool(status.Connected))
	fmt.Printf("Peer Count: %d\n", status.PeerCount)
	fmt.Printf("Database Size: %s\n", formatBytes(status.DatabaseSize))
	fmt.Printf("Uptime: %v\n", status.Uptime.Round(time.Second))
}

func printQueryResult(result *client.QueryResult) {
	fmt.Printf("📊 Query Result\n")
	fmt.Printf("Rows: %d\n\n", result.Count)

	if len(result.Rows) == 0 {
		fmt.Printf("No data returned\n")
		return
	}

	// Print header
	for i, col := range result.Columns {
		if i > 0 {
			fmt.Printf(" | ")
		}
		fmt.Printf("%-15s", col)
	}
	fmt.Println()

	// Print separator
	for i := range result.Columns {
		if i > 0 {
			fmt.Printf("-+-")
		}
		fmt.Printf("%-15s", "---------------")
	}
	fmt.Println()

	// Print rows
	for _, row := range result.Rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Printf(" | ")
			}
			fmt.Printf("%-15v", cell)
		}
		fmt.Println()
	}
}

func printJSON(data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		return
	}
	fmt.Println(string(jsonData))
}

// Helper functions

func getStatusEmoji(status string) string {
	switch status {
	case "healthy":
		return "🟢 "
	case "degraded":
		return "🟡 "
	case "unhealthy":
		return "🔴 "
	default:
		return "⚪ "
	}
}

func getBoolEmoji(b bool) string {
	if b {
		return "✅ "
	}
	return "❌ "
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// extractPeerIDFromFile extracts peer ID from an identity key file
func extractPeerIDFromFile(keyFile string) string {
	// Read the identity key file
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return ""
	}

	// Unmarshal the private key
	priv, err := crypto.UnmarshalPrivateKey(data)
	if err != nil {
		return ""
	}

	// Get the public key
	pub := priv.GetPublic()

	// Get the peer ID
	peerID, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return ""
	}

	return peerID.String()
}

// extractPeerIDFromMultiaddr extracts the peer ID from a multiaddr string
func extractPeerIDFromMultiaddr(multiaddr string) string {
	// Look for /p2p/ followed by the peer ID
	parts := strings.Split(multiaddr, "/p2p/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}
