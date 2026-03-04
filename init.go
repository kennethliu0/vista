package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Docker Compose types

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Build   interface{} `yaml:"build"`
	Command interface{} `yaml:"command"`
	Image   string      `yaml:"image"`
}

// autoDiscover returns the path of the first recognized config file found in
// the current directory, checking in priority order.
func autoDiscover() string {
	for _, name := range []string{
		"Procfile",
		"compose.yaml",
		"compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return ""
}

// detectFormat returns the format implied by a filename.
func detectFormat(name string) (string, error) {
	base := filepath.Base(name)
	switch {
	case base == "Procfile":
		return "procfile", nil
	case strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml"):
		return "compose", nil
	default:
		return "", fmt.Errorf("unrecognized file %q — expected a Procfile or a .yaml/.yml compose file", base)
	}
}

// parseCompose extracts serviceConfigs from a Docker Compose YAML file.
func parseCompose(path string, cwd string) ([]serviceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(cf.Services) == 0 {
		return nil, fmt.Errorf("no services found in %s", path)
	}

	// Sort names alphabetically for deterministic output.
	names := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]serviceConfig, len(names))
	for i, name := range names {
		services[i] = serviceConfig{
			Name: name,
			Cmd:  "docker compose up " + name,
			Dir:  cwd,
		}
	}
	return services, nil
}

// parseProcfile extracts serviceConfigs from a Procfile.
// Each non-blank, non-comment line has the form: name: command
func parseProcfile(path string, cwd string) ([]serviceConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()

	var services []serviceConfig
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			return nil, fmt.Errorf("%s line %d: expected \"name: command\", got %q", path, lineNum, line)
		}
		name := strings.TrimSpace(line[:idx])
		cmd := strings.TrimSpace(line[idx+1:])
		if name == "" || cmd == "" {
			return nil, fmt.Errorf("%s line %d: name and command must both be non-empty", path, lineNum)
		}
		services = append(services, serviceConfig{Name: name, Cmd: cmd, Dir: cwd})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("no services found in %s", path)
	}
	return services, nil
}

func runInit(file string) error {
	path := file
	if path == "" {
		path = autoDiscover()
		if path == "" {
			return fmt.Errorf("no config file found (checked Procfile, compose.yaml, compose.yml, docker-compose.yaml, docker-compose.yml)")
		}
	} else {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("file %q not found", path)
		}
	}

	format, err := detectFormat(path)
	if err != nil {
		return err
	}

	if _, err := os.Stat("vista.json"); err == nil {
		return fmt.Errorf("vista.json already exists")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	var services []serviceConfig
	switch format {
	case "compose":
		services, err = parseCompose(path, cwd)
	case "procfile":
		services, err = parseProcfile(path, cwd)
	}
	if err != nil {
		return err
	}

	cfg := configFile{
		Profiles: []profileConfig{
			{
				Name:    "default",
				Default: true,
				Services: services,
				GlobalViews: []globalViewConfig{
					{Name: "All", Services: []string{}},
				},
			},
		},
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	if err := os.WriteFile("vista.json", append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing vista.json: %w", err)
	}

	fmt.Printf("Created vista.json from %s with %d service(s): ", path, len(services))
	for i, s := range services {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(s.Name)
	}
	fmt.Println()

	return nil
}
