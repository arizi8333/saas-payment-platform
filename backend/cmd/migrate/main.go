package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/saas-payment-platform/backend/internal/config"
	"github.com/saas-payment-platform/backend/internal/pkg/database"
	"github.com/saas-payment-platform/backend/internal/pkg/logger"
)

func main() {
	logger.Setup()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	cfg := config.Load()

	db, err := database.NewPostgresDB(&cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	migrator := database.NewMigrator(db, "./migrations")

	switch command {
	case "up":
		if err := migrator.MigrateUp(); err != nil {
			slog.Error("migration up failed", "error", err)
			os.Exit(1)
		}
	case "down":
		steps := 1
		if len(os.Args) >= 3 {
			s, err := strconv.Atoi(os.Args[2])
			if err != nil {
				slog.Error("invalid steps argument", "value", os.Args[2], "error", err)
				os.Exit(1)
			}
			steps = s
		}
		if err := migrator.MigrateDown(steps); err != nil {
			slog.Error("migration down failed", "error", err)
			os.Exit(1)
		}
	case "status":
		migrations, err := migrator.Status()
		if err != nil {
			slog.Error("failed to get migration status", "error", err)
			os.Exit(1)
		}
		if len(migrations) == 0 {
			fmt.Println("No migrations applied.")
			return
		}
		fmt.Printf("%-60s %s\n", "VERSION", "APPLIED AT")
		fmt.Println(repeat("-", 90))
		for _, mg := range migrations {
			fmt.Printf("%-60s %s\n", mg.Version, mg.AppliedAt.Format("2006-01-02 15:04:05"))
		}
	default:
		slog.Error("unknown command", "command", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: migrate <command> [args]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  up              Apply all pending migrations\n")
	fmt.Fprintf(os.Stderr, "  down [steps]    Roll back the last N migrations (default: 1)\n")
	fmt.Fprintf(os.Stderr, "  status          Show applied migrations\n")
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
