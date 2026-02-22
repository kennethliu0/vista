package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Build   interface{} `yaml:"build"`
	Command interface{} `yaml:"command"`
	Image   string      `yaml:"image"`
}

func runInit(file string) error {
	var composePath string
	if file != "" {
		if _, err := os.Stat(file); err != nil {
			return fmt.Errorf("file %q not found", file)
		}
		composePath = file
	} else {
		for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
			if _, err := os.Stat(name); err == nil {
				composePath = name
				break
			}
		}
		if composePath == "" {
			return fmt.Errorf("no compose file found (checked compose.yaml, compose.yml, docker-compose.yaml, docker-compose.yml)")
		}
	}

	if _, err := os.Stat("vista.json"); err == nil {
		return fmt.Errorf("vista.json already exists")
	}

	data, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", composePath, err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("parsing %s: %w", composePath, err)
	}

	if len(cf.Services) == 0 {
		return fmt.Errorf("no services found in %s", composePath)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Collect service names in a stable order by iterating the map.
	// yaml.v3 preserves document order via the Decoder but Unmarshal into a
	// map loses it; sort alphabetically for deterministic output.
	names := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		names = append(names, name)
	}
	// Sort alphabetically for deterministic output.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}

	services := make([]serviceConfig, len(names))
	for i, name := range names {
		services[i] = serviceConfig{
			Name: name,
			Cmd:  "docker compose up " + name,
			Dir:  cwd,
		}
	}

	cfg := configFile{
		Services: services,
		GlobalViews: []globalViewConfig{
			{Name: "All", Services: []string{}},
		},
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	if err := os.WriteFile("vista.json", append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing vista.json: %w", err)
	}

	fmt.Printf("Created vista.json from %s with %d service(s): ", composePath, len(services))
	for i, s := range services {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(s.Name)
	}
	fmt.Println()

	return nil
}
