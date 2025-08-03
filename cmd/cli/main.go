package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"git.debros.io/DeBros/network/pkg/client"
)

var (
	bootstrapPeer = "/ip4/127.0.0.1/tcp/4001"
	timeout       = 30 * time.Second
	format        = "table"
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

	switch command {
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
	case "help", "--help", "-h":
		showHelp()
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

func createClient() (client.NetworkClient, error) {
	// Try to discover the bootstrap peer from saved peer info
	discoveredPeer := discoverBootstrapPeer()
	if discoveredPeer != "" {
		bootstrapPeer = discoveredPeer
	}

	config := client.DefaultClientConfig("network-cli")
	config.BootstrapPeers = []string{bootstrapPeer}
	config.ConnectTimeout = timeout
	config.QuietMode = true // Suppress debug/info logs for CLI

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
	fmt.Printf("  query <sql>               - Execute database query\n")
	fmt.Printf("  storage get <key>         - Get value from storage\n")
	fmt.Printf("  storage put <key> <value> - Store value in storage\n")
	fmt.Printf("  storage list [prefix]     - List storage keys\n")
	fmt.Printf("  pubsub publish <topic> <msg> - Publish message\n")
	fmt.Printf("  pubsub subscribe <topic> [duration] - Subscribe to topic\n")
	fmt.Printf("  pubsub topics             - List topics\n")
	fmt.Printf("  connect <peer_address>    - Connect to peer\n")
	fmt.Printf("  help                      - Show this help\n\n")
	fmt.Printf("Global Flags:\n")
	fmt.Printf("  -b, --bootstrap <addr>    - Bootstrap peer address (default: /ip4/127.0.0.1/tcp/4001)\n")
	fmt.Printf("  -f, --format <format>     - Output format: table, json (default: table)\n")
	fmt.Printf("  -t, --timeout <duration>  - Operation timeout (default: 30s)\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  network-cli health\n")
	fmt.Printf("  network-cli peers --format json\n")
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
