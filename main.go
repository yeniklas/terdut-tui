package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/terdut-tui/internal/api"
	"github.com/yeniklas/terdut-tui/internal/config"
	"github.com/yeniklas/terdut-tui/internal/tui"
	"github.com/yeniklas/terdut-tui/internal/updater"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	updateFlag := flag.Bool("self-update", false, "update terdut-tui to the latest release")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if *updateFlag {
		if err := updater.Run(version); err != nil {
			fmt.Fprintln(os.Stderr, "update:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := api.NewClient(cfg.ServerURL, cfg.APIKey)
	model := tui.NewModel(client, cfg.ServerURL, cfg.RefreshInterval)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
