package cli

import (
	"fmt"
	"os"

	"github.com/DeBrosOfficial/network/pkg/auth"
)

// HandleAuthCommand handles authentication commands
func HandleAuthCommand(args []string) {
	if len(args) == 0 {
		showAuthHelp()
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "login":
		handleAuthLogin()
	case "logout":
		handleAuthLogout()
	case "whoami":
		handleAuthWhoami()
	case "status":
		handleAuthStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown auth command: %s\n", subcommand)
		showAuthHelp()
		os.Exit(1)
	}
}

func showAuthHelp() {
	fmt.Printf("🔐 Authentication Commands\n\n")
	fmt.Printf("Usage: network-cli auth <subcommand>\n\n")
	fmt.Printf("Subcommands:\n")
	fmt.Printf("  login      - Authenticate with wallet\n")
	fmt.Printf("  logout     - Clear stored credentials\n")
	fmt.Printf("  whoami     - Show current authentication status\n")
	fmt.Printf("  status     - Show detailed authentication info\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  network-cli auth login\n")
	fmt.Printf("  network-cli auth whoami\n")
	fmt.Printf("  network-cli auth status\n")
	fmt.Printf("  network-cli auth logout\n\n")
	fmt.Printf("Environment Variables:\n")
	fmt.Printf("  DEBROS_GATEWAY_URL - Gateway URL (overrides environment config)\n\n")
	fmt.Printf("Note: Authentication uses the currently active environment.\n")
	fmt.Printf("      Use 'network-cli env current' to see your active environment.\n")
}

func handleAuthLogin() {
	gatewayURL := getGatewayURL()
	fmt.Printf("🔐 Authenticating with gateway at: %s\n", gatewayURL)

	// Use the wallet authentication flow
	creds, err := auth.PerformWalletAuthentication(gatewayURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Authentication failed: %v\n", err)
		os.Exit(1)
	}

	// Save credentials to file
	if err := auth.SaveCredentialsForDefaultGateway(creds); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to save credentials: %v\n", err)
		os.Exit(1)
	}

	credsPath, _ := auth.GetCredentialsPath()
	fmt.Printf("✅ Authentication successful!\n")
	fmt.Printf("📁 Credentials saved to: %s\n", credsPath)
	fmt.Printf("🎯 Wallet: %s\n", creds.Wallet)
	fmt.Printf("🏢 Namespace: %s\n", creds.Namespace)
}

func handleAuthLogout() {
	if err := auth.ClearAllCredentials(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to clear credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Logged out successfully - all credentials have been cleared")
}

func handleAuthWhoami() {
	store, err := auth.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load credentials: %v\n", err)
		os.Exit(1)
	}

	gatewayURL := getGatewayURL()
	creds, exists := store.GetCredentialsForGateway(gatewayURL)

	if !exists || !creds.IsValid() {
		fmt.Println("❌ Not authenticated - run 'network-cli auth login' to authenticate")
		os.Exit(1)
	}

	fmt.Println("✅ Authenticated")
	fmt.Printf("  Wallet:    %s\n", creds.Wallet)
	fmt.Printf("  Namespace: %s\n", creds.Namespace)
	fmt.Printf("  Issued At: %s\n", creds.IssuedAt.Format("2006-01-02 15:04:05"))
	if !creds.ExpiresAt.IsZero() {
		fmt.Printf("  Expires At: %s\n", creds.ExpiresAt.Format("2006-01-02 15:04:05"))
	}
	if !creds.LastUsedAt.IsZero() {
		fmt.Printf("  Last Used: %s\n", creds.LastUsedAt.Format("2006-01-02 15:04:05"))
	}
	if creds.Plan != "" {
		fmt.Printf("  Plan:      %s\n", creds.Plan)
	}
}

func handleAuthStatus() {
	store, err := auth.LoadCredentials()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load credentials: %v\n", err)
		os.Exit(1)
	}

	gatewayURL := getGatewayURL()
	creds, exists := store.GetCredentialsForGateway(gatewayURL)

	// Show active environment
	env, err := GetActiveEnvironment()
	if err == nil {
		fmt.Printf("🌍 Active Environment: %s\n", env.Name)
	}

	fmt.Println("🔐 Authentication Status")
	fmt.Printf("  Gateway URL: %s\n", gatewayURL)

	if !exists || creds == nil {
		fmt.Println("  Status:     ❌ Not authenticated")
		return
	}

	if !creds.IsValid() {
		fmt.Println("  Status:     ⚠️  Credentials expired")
		if !creds.ExpiresAt.IsZero() {
			fmt.Printf("  Expired At: %s\n", creds.ExpiresAt.Format("2006-01-02 15:04:05"))
		}
		return
	}

	fmt.Println("  Status:     ✅ Authenticated")
	fmt.Printf("  Wallet:     %s\n", creds.Wallet)
	fmt.Printf("  Namespace:  %s\n", creds.Namespace)
	if !creds.ExpiresAt.IsZero() {
		fmt.Printf("  Expires:    %s\n", creds.ExpiresAt.Format("2006-01-02 15:04:05"))
	}
	if !creds.LastUsedAt.IsZero() {
		fmt.Printf("  Last Used:  %s\n", creds.LastUsedAt.Format("2006-01-02 15:04:05"))
	}
}

// getGatewayURL returns the gateway URL based on environment or env var
func getGatewayURL() string {
	// Check environment variable first (for backwards compatibility)
	if url := os.Getenv("DEBROS_GATEWAY_URL"); url != "" {
		return url
	}

	// Get from active environment
	env, err := GetActiveEnvironment()
	if err == nil {
		return env.GatewayURL
	}

	// Fallback to default
	return "http://localhost:6001"
}
