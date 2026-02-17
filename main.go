package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type configFile struct {
	Services []serviceConfig `json:"services"`
}

type serviceConfig struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
	Dir  string `json:"dir"`
}

func loadConfig() ([]*Service, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	path := filepath.Join(home, ".config", "vista", "vista.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("no services defined in %s", path)
	}

	services := make([]*Service, len(cfg.Services))
	for i, sc := range cfg.Services {
		services[i] = &Service{Name: sc.Name, Cmd: sc.Cmd, Dir: sc.Dir}
	}
	return services, nil
}

func main() {
	services, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logCh := make(chan logMsg, 256)
	m := newModel(services, logCh)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
