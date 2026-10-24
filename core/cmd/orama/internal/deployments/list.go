package deployments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Orama-Network/orama/core/cmd/orama/internal/clierr"
	"github.com/Orama-Network/orama/core/cmd/orama/internal/printer"
	"github.com/spf13/cobra"
)

// ListCmd lists all deployments
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deployments",
	RunE:  listDeployments,
}

// GetCmd gets a specific deployment
var GetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get deployment details",
	Args:  cobra.ExactArgs(1),
	RunE:  getDeployment,
}

// DeleteCmd deletes a deployment
var DeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a deployment",
	Args:  cobra.ExactArgs(1),
	RunE:  deleteDeployment,
}

// RollbackCmd rolls back a deployment
var RollbackCmd = &cobra.Command{
	Use:   "rollback <name>",
	Short: "Rollback a deployment to a previous version",
	Args:  cobra.ExactArgs(1),
	RunE:  rollbackDeployment,
}

var (
	rollbackVersion int
	rollbackYes     bool
	deleteYes       bool
)

func init() {
	RollbackCmd.Flags().IntVar(&rollbackVersion, "version", 0, "Version to rollback to (required)")
	RollbackCmd.MarkFlagRequired("version")
	RollbackCmd.Flags().BoolVar(&rollbackYes, "yes", false, "Roll back without asking")
	DeleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "Delete without asking")
}

// confirmYN asks prompt on out and takes y or yes from in, unless yes is set.
// Any other answer, end of input included, is an Aborted error, so a script
// that does not confirm exits non-zero instead of reading "nothing happened"
// as success.
func confirmYN(in io.Reader, out io.Writer, yes bool, prompt string) error {
	if yes {
		return nil
	}
	fmt.Fprint(out, prompt)
	return clierr.Confirm(in, "y", "Y", "yes")
}

func listDeployments(cmd *cobra.Command, args []string) error {
	apiURL, err := getAPIURL()
	if err != nil {
		return err
	}
	url := apiURL + "/v1/deployments/list"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	token, err := getAuthToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list deployments: %s", string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	out := printer.For(cmd)
	if out.JSONMode() {
		// The gateway's reply verbatim, so a field the CLI does not render is
		// still there for whatever is parsing it.
		fmt.Fprintln(out.Out(), string(body))
		return nil
	}

	deployments, ok := result["deployments"].([]interface{})
	if !ok || len(deployments) == 0 {
		out.Printf("No deployments found\n")
		return nil
	}

	rows := make([][]string, 0, len(deployments))
	for _, dep := range deployments {
		d, _ := dep.(map[string]interface{})
		createdAt := ""
		if created, ok := d["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, created); err == nil {
				createdAt = t.Format("2006-01-02 15:04")
			}
		}
		rows = append(rows, []string{
			fmt.Sprint(d["name"]), fmt.Sprint(d["type"]), fmt.Sprint(d["status"]),
			fmt.Sprint(d["version"]), createdAt,
		})
	}

	if err := out.Table([]string{"NAME", "TYPE", "STATUS", "VERSION", "CREATED"}, rows); err != nil {
		return err
	}
	out.Printf("\nTotal: %v\n", result["total"])
	return nil
}

func getDeployment(cmd *cobra.Command, args []string) error {
	name := args[0]

	apiURL, err := getAPIURL()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1/deployments/get?name=%s", apiURL, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	token, err := getAuthToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get deployment: %s", string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	// Print deployment info
	fmt.Printf("Deployment: %s\n\n", result["name"])
	fmt.Printf("ID:               %s\n", result["id"])
	fmt.Printf("Type:             %s\n", result["type"])
	fmt.Printf("Status:           %s\n", result["status"])
	fmt.Printf("Version:          %v\n", result["version"])
	fmt.Printf("Namespace:        %s\n", result["namespace"])

	if contentCID, ok := result["content_cid"]; ok && contentCID != "" {
		fmt.Printf("Content CID:      %s\n", contentCID)
	}
	if buildCID, ok := result["build_cid"]; ok && buildCID != "" {
		fmt.Printf("Build CID:        %s\n", buildCID)
	}

	if port, ok := result["port"]; ok && port != nil && port.(float64) > 0 {
		fmt.Printf("Port:             %v\n", port)
	}

	if homeNodeID, ok := result["home_node_id"]; ok && homeNodeID != "" {
		fmt.Printf("Home Node:        %s\n", homeNodeID)
	}

	if subdomain, ok := result["subdomain"]; ok && subdomain != "" {
		fmt.Printf("Subdomain:        %s\n", subdomain)
	}

	fmt.Printf("Memory Limit:     %v MB\n", result["memory_limit_mb"])
	fmt.Printf("CPU Limit:        %v%%\n", result["cpu_limit_percent"])
	fmt.Printf("Restart Policy:   %s\n", result["restart_policy"])

	if urls, ok := result["urls"].([]interface{}); ok && len(urls) > 0 {
		fmt.Printf("\nURLs:\n")
		for _, url := range urls {
			fmt.Printf("  • %s\n", url)
		}
	}

	if createdAt, ok := result["created_at"].(string); ok {
		fmt.Printf("\nCreated:          %s\n", createdAt)
	}
	if updatedAt, ok := result["updated_at"].(string); ok {
		fmt.Printf("Updated:          %s\n", updatedAt)
	}

	return nil
}

func deleteDeployment(cmd *cobra.Command, args []string) error {
	name := args[0]

	prompt := fmt.Sprintf("⚠️  Are you sure you want to delete deployment '%s'? (y/N): ", name)
	if err := confirmYN(cmd.InOrStdin(), cmd.OutOrStdout(), deleteYes, prompt); err != nil {
		return err
	}

	apiURL, err := getAPIURL()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1/deployments/delete?name=%s", apiURL, name)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	token, err := getAuthToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete deployment: %s", string(body))
	}

	fmt.Printf("✅ Deployment '%s' deleted successfully\n", name)

	return nil
}

func rollbackDeployment(cmd *cobra.Command, args []string) error {
	name := args[0]

	if rollbackVersion <= 0 {
		return fmt.Errorf("version must be positive")
	}

	prompt := fmt.Sprintf("⚠️  Rolling back '%s' to version %d. Continue? (y/N): ", name, rollbackVersion)
	if err := confirmYN(cmd.InOrStdin(), cmd.OutOrStdout(), rollbackYes, prompt); err != nil {
		return err
	}

	apiURL, err := getAPIURL()
	if err != nil {
		return err
	}
	url := apiURL + "/v1/deployments/rollback?name=" + name

	payload := map[string]interface{}{
		"name":    name,
		"version": rollbackVersion,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	token, err := getAuthToken()
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rollback failed: %s", string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	fmt.Printf("\n✅ Rollback successful!\n\n")
	fmt.Printf("Deployment:       %s\n", result["name"])
	fmt.Printf("Current Version:  %v\n", result["version"])
	fmt.Printf("Rolled Back From: %v\n", result["rolled_back_from"])
	fmt.Printf("Rolled Back To:   %v\n", result["rolled_back_to"])
	fmt.Printf("Status:           %s\n", result["status"])

	return nil
}
