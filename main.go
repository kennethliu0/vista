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

func loadVistaJSON(path string) ([]*Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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

func loadConfig() ([]*Service, error) {
	// 1. Local ./vista.json
	if svcs, err := loadVistaJSON("vista.json"); err == nil {
		return svcs, nil
	}

	// 2. Global ~/.config/vista/vista.json
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	globalPath := filepath.Join(home, ".config", "vista", "vista.json")
	if svcs, err := loadVistaJSON(globalPath); err == nil {
		return svcs, nil
	}

	return nil, fmt.Errorf("no config found (checked ./vista.json and %s)", globalPath)
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
