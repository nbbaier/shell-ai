package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"q/logger"
	. "q/types"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	limitFlag  int
	jsonFlag   bool
	pathFlag   bool
	statusFlag bool
)

// LogsCmd is the root command for logs operations
var LogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View and manage request logs",
	Long:  "View recent API requests, token usage, and costs stored in the SQLite database",
	Run:   runLogsCommand,
}

func init() {
	LogsCmd.Flags().IntVarP(&limitFlag, "limit", "n", 3, "Number of recent entries to display")
	LogsCmd.Flags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	LogsCmd.Flags().BoolVar(&pathFlag, "path", false, "Show the path to the logs database")
	LogsCmd.Flags().BoolVar(&statusFlag, "status", false, "Show database statistics")
}

func runLogsCommand(cmd *cobra.Command, args []string) {
	log, err := logger.NewRequestLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening logs database: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Handle --path flag
	if pathFlag {
		fmt.Println(log.GetDBPath())
		return
	}

	// Handle --status flag
	if statusFlag {
		printStatus(log)
		return
	}

	// Default: show recent logs
	entries, err := log.GetRecentResponses(limitFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving logs: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No logs found. Make some requests to see them here!")
		return
	}

	if jsonFlag {
		printJSON(entries)
	} else {
		printFormatted(entries)
	}
}

func printJSON(entries []LogEntry) {
	for _, entry := range entries {
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			continue
		}
		fmt.Println(string(data))
	}
}

func printFormatted(entries []LogEntry) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	valueStyle := lipgloss.NewStyle()
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	for i, entry := range entries {
		// Header with timestamp and model
		header := fmt.Sprintf("Entry %d - %s [%s]",
			len(entries)-i,
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Model)
		fmt.Println(headerStyle.Render(header))
		fmt.Println()

		// Prompt
		fmt.Print(labelStyle.Render("Prompt: "))
		for _, msg := range entry.Messages {
			if msg.Role == "user" {
				fmt.Println(valueStyle.Render(msg.Content))
				break
			}
		}
		fmt.Println()

		// Response
		fmt.Print(labelStyle.Render("Response: "))
		if entry.Error != "" {
			fmt.Println(errorStyle.Render("ERROR: " + entry.Error))
		} else {
			// Truncate long responses
			response := entry.Response
			if len(response) > 500 {
				response = response[:497] + "..."
			}
			// Highlight code blocks
			if strings.Contains(response, "```") {
				fmt.Println(codeStyle.Render(response))
			} else {
				fmt.Println(valueStyle.Render(response))
			}
		}
		fmt.Println()

		// Metadata
		fmt.Print(labelStyle.Render("Tokens: "))
		fmt.Printf("%d input + %d output = %d total\n",
			entry.PromptTokens, entry.CompletionTokens, entry.TotalTokens)

		fmt.Print(labelStyle.Render("Cost: "))
		fmt.Printf("$%.6f\n", entry.EstimatedCost)

		if entry.DurationMs > 0 {
			fmt.Print(labelStyle.Render("Duration: "))
			fmt.Printf("%dms\n", entry.DurationMs)
		}

		if entry.RequestID != "" {
			fmt.Print(labelStyle.Render("Request ID: "))
			fmt.Println(entry.RequestID)
		}

		// Divider
		if i < len(entries)-1 {
			fmt.Println(dividerStyle.Render(strings.Repeat("─", 80)))
			fmt.Println()
		}
	}
}

func printStatus(log *logger.RequestLogger) {
	// For now, just show the database path and basic info
	fmt.Println("Database path:", log.GetDBPath())

	// Try to get some stats
	entries, err := log.GetRecentResponses(1000000) // Get all entries
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading database: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println("Total requests: 0")
		return
	}

	totalTokens := 0
	totalCost := 0.0
	modelCounts := make(map[string]int)

	for _, entry := range entries {
		totalTokens += entry.TotalTokens
		totalCost += entry.EstimatedCost
		modelCounts[entry.Model]++
	}

	fmt.Printf("Total requests: %d\n", len(entries))
	fmt.Printf("Total tokens: %d\n", totalTokens)
	fmt.Printf("Total estimated cost: $%.6f\n", totalCost)
	fmt.Println("\nRequests by model:")
	for model, count := range modelCounts {
		fmt.Printf("  %s: %d\n", model, count)
	}
}
