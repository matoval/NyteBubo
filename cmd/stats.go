package cmd

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"NyteBubo/internal/core"
	"NyteBubo/internal/types"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	exportCSV bool
	csvFile   string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "View token usage statistics for issues",
	Long:  `Display token usage and cost statistics for all processed issues. Optionally export to CSV.`,
	Run:   runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().BoolVarP(&exportCSV, "export", "e", false, "Export statistics to CSV file")
	statsCmd.Flags().StringVarP(&csvFile, "file", "f", "usage_stats.csv", "CSV file name for export")
}

func runStats(cmd *cobra.Command, args []string) {
	// Load configuration
	config := types.Config{
		StateDBPath: "./agent_state.db",
	}

	configPath := "config.yaml"
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to read config.yaml: %v", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Fatalf("Failed to parse config.yaml: %v", err)
		}
	}

	// Open state manager
	stateManager, err := core.NewStateManager(config.StateDBPath)
	if err != nil {
		log.Fatalf("Failed to open state database: %v", err)
	}
	defer stateManager.Close()

	// Get all issues with stats
	states, err := stateManager.GetAllIssuesWithStats()
	if err != nil {
		log.Fatalf("Failed to get statistics: %v", err)
	}

	if len(states) == 0 {
		fmt.Println("No issues found in database.")
		return
	}

	// Display statistics
	displayStats(states)

	// Export to CSV if requested
	if exportCSV {
		if err := exportToCSV(states, csvFile); err != nil {
			log.Fatalf("Failed to export to CSV: %v", err)
		}
		fmt.Printf("\n✅ Statistics exported to: %s\n", csvFile)
	}
}

func displayStats(states []core.State) {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                     Token Usage Statistics                             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝\n")

	var totalInputTokens int64
	var totalOutputTokens int64
	var totalCost float64

	fmt.Printf("%-30s %-12s %-12s %-10s %s\n", "Issue", "Input Tokens", "Output Tokens", "Cost", "Status")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")

	for _, state := range states {
		issueID := fmt.Sprintf("%s/%s#%d", state.Owner, state.Repo, state.IssueNumber)
		fmt.Printf("%-30s %12d %12d  $%8.4f  %s\n",
			issueID,
			state.TotalInputTokens,
			state.TotalOutputTokens,
			state.TotalCost,
			state.Status,
		)

		totalInputTokens += state.TotalInputTokens
		totalOutputTokens += state.TotalOutputTokens
		totalCost += state.TotalCost
	}

	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-30s %12d %12d  $%8.4f\n",
		"TOTAL",
		totalInputTokens,
		totalOutputTokens,
		totalCost,
	)

	// Summary statistics
	avgCostPerIssue := totalCost / float64(len(states))
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Issues: %d\n", len(states))
	fmt.Printf("  Total Tokens: %d (input) + %d (output) = %d total\n",
		totalInputTokens, totalOutputTokens, totalInputTokens+totalOutputTokens)
	fmt.Printf("  Total Cost: $%.4f\n", totalCost)
	fmt.Printf("  Average Cost per Issue: $%.4f\n", avgCostPerIssue)
	fmt.Println()
}

func exportToCSV(states []core.State, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Owner",
		"Repository",
		"Issue Number",
		"Status",
		"PR Number",
		"Input Tokens",
		"Output Tokens",
		"Total Tokens",
		"Cost",
		"Created At",
		"Updated At",
		"Completed At",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, state := range states {
		prNumber := ""
		if state.PRNumber != nil {
			prNumber = fmt.Sprintf("%d", *state.PRNumber)
		}

		completedAt := ""
		if state.CompletedAt != nil {
			completedAt = state.CompletedAt.Format("2006-01-02 15:04:05")
		}

		row := []string{
			state.Owner,
			state.Repo,
			fmt.Sprintf("%d", state.IssueNumber),
			state.Status,
			prNumber,
			fmt.Sprintf("%d", state.TotalInputTokens),
			fmt.Sprintf("%d", state.TotalOutputTokens),
			fmt.Sprintf("%d", state.TotalInputTokens+state.TotalOutputTokens),
			fmt.Sprintf("%.4f", state.TotalCost),
			state.CreatedAt.Format("2006-01-02 15:04:05"),
			state.UpdatedAt.Format("2006-01-02 15:04:05"),
			completedAt,
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}
