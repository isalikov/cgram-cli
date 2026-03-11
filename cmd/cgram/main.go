package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isalikov/cgram-cli/internal/client"
	"github.com/isalikov/cgram-cli/internal/config"
	"github.com/isalikov/cgram-cli/internal/tui"
)

func main() {
	cfg := config.Load()

	cl := client.New(cfg.ServerAddr)

	app := tui.NewApp(cl, cfg.DataDir, cfg.ServerAddr)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
