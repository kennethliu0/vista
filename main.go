package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type configFile struct {
	Services    []serviceConfig    `json:"services"`
	GlobalViews []globalViewConfig `json:"globalViews"`
}

type serviceConfig struct {
	Name string `json:"name"`
	Cmd  string `json:"cmd"`
	Dir  string `json:"dir"`
}

type globalViewConfig struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

func loadVistaJSON(path string) ([]*Service, []*globalView, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	if len(cfg.Services) == 0 {
		return nil, nil, fmt.Errorf("no services defined in %s", path)
	}

	services := make([]*Service, len(cfg.Services))
	serviceNames := make(map[string]bool)
	for i, sc := range cfg.Services {
		if serviceNames[sc.Name] {
			return nil, nil, fmt.Errorf("duplicate service name %q in %s", sc.Name, path)
		}
		serviceNames[sc.Name] = true
		if sc.Dir != "" {
			if _, err := os.Stat(sc.Dir); os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("service %q: directory %q does not exist", sc.Name, sc.Dir)
			}
		}
		services[i] = &Service{Name: sc.Name, Cmd: sc.Cmd, Dir: sc.Dir}
	}

	var views []*globalView
	for _, gvc := range cfg.GlobalViews {
		gv := &globalView{
			name:     gvc.Name,
			services: make(map[string]bool),
		}
		if len(gvc.Services) == 0 {
			// Empty list means all services enabled
			for name := range serviceNames {
				gv.services[name] = true
			}
		} else {
			// Start with all disabled, enable only listed ones
			for name := range serviceNames {
				gv.services[name] = false
			}
			for _, name := range gvc.Services {
				if serviceNames[name] {
					gv.services[name] = true
				}
			}
		}
		views = append(views, gv)
	}

	return services, views, nil
}

func loadConfig() ([]*Service, []*globalView, error) {
	// 1. Local ./vista.json
	svcs, views, err := loadVistaJSON("vista.json")
	if err == nil {
		return svcs, views, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}

	// 2. Global ~/.config/vista/vista.json
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	globalPath := filepath.Join(home, ".config", "vista", "vista.json")
	svcs, views, err = loadVistaJSON(globalPath)
	if err == nil {
		return svcs, views, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, err
	}

	return nil, nil, fmt.Errorf("no config found (checked ./vista.json and %s)", globalPath)
}

func main() {
	services, views, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logCh := make(chan tea.Msg, 256)
	m := newModel(services, views, logCh)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
