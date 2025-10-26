package cli

import (
	"fmt"
	"os"
)

// HandleEnvCommand handles the 'env' command and its subcommands
func HandleEnvCommand(args []string) {
	if len(args) == 0 {
		showEnvHelp()
		return
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "list":
		handleEnvList()
	case "current":
		handleEnvCurrent()
	case "switch":
		handleEnvSwitch(subargs)
	case "enable":
		handleEnvEnable(subargs)
	case "help":
		showEnvHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown env subcommand: %s\n", subcommand)
		showEnvHelp()
		os.Exit(1)
	}
}

func showEnvHelp() {
	fmt.Printf("🌍 Environment Management Commands\n\n")
	fmt.Printf("Usage: network-cli env <subcommand>\n\n")
	fmt.Printf("Subcommands:\n")
	fmt.Printf("  list       - List all available environments\n")
	fmt.Printf("  current    - Show current active environment\n")
	fmt.Printf("  switch     - Switch to a different environment\n")
	fmt.Printf("  enable     - Alias for 'switch' (e.g., 'devnet enable')\n\n")
	fmt.Printf("Available Environments:\n")
	fmt.Printf("  local      - Local development (http://localhost:6001)\n")
	fmt.Printf("  devnet     - Development network (https://devnet.debros.network)\n")
	fmt.Printf("  testnet    - Test network (https://testnet.debros.network)\n\n")
	fmt.Printf("Examples:\n")
	fmt.Printf("  network-cli env list\n")
	fmt.Printf("  network-cli env current\n")
	fmt.Printf("  network-cli env switch devnet\n")
	fmt.Printf("  network-cli env enable testnet\n")
	fmt.Printf("  network-cli devnet enable      # Shorthand for switch to devnet\n")
	fmt.Printf("  network-cli testnet enable     # Shorthand for switch to testnet\n")
}

func handleEnvList() {
	// Initialize environments if needed
	if err := InitializeEnvironments(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize environments: %v\n", err)
		os.Exit(1)
	}

	envConfig, err := LoadEnvironmentConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load environment config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🌍 Available Environments:\n\n")
	for _, env := range envConfig.Environments {
		active := ""
		if env.Name == envConfig.ActiveEnvironment {
			active = " ✅ (active)"
		}
		fmt.Printf("  %s%s\n", env.Name, active)
		fmt.Printf("    Gateway: %s\n", env.GatewayURL)
		fmt.Printf("    Description: %s\n\n", env.Description)
	}
}

func handleEnvCurrent() {
	// Initialize environments if needed
	if err := InitializeEnvironments(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize environments: %v\n", err)
		os.Exit(1)
	}

	env, err := GetActiveEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to get active environment: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Current Environment: %s\n", env.Name)
	fmt.Printf("   Gateway URL: %s\n", env.GatewayURL)
	fmt.Printf("   Description: %s\n", env.Description)
}

func handleEnvSwitch(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: network-cli env switch <environment>\n")
		fmt.Fprintf(os.Stderr, "Available: local, devnet, testnet\n")
		os.Exit(1)
	}

	envName := args[0]

	// Initialize environments if needed
	if err := InitializeEnvironments(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize environments: %v\n", err)
		os.Exit(1)
	}

	// Get old environment
	oldEnv, _ := GetActiveEnvironment()

	// Switch environment
	if err := SwitchEnvironment(envName); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to switch environment: %v\n", err)
		os.Exit(1)
	}

	// Get new environment
	newEnv, err := GetActiveEnvironment()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to get new environment: %v\n", err)
		os.Exit(1)
	}

	if oldEnv != nil && oldEnv.Name != newEnv.Name {
		fmt.Printf("✅ Switched environment: %s → %s\n", oldEnv.Name, newEnv.Name)
	} else {
		fmt.Printf("✅ Environment set to: %s\n", newEnv.Name)
	}
	fmt.Printf("   Gateway URL: %s\n", newEnv.GatewayURL)
}

func handleEnvEnable(args []string) {
	// 'enable' is just an alias for 'switch'
	handleEnvSwitch(args)
}
