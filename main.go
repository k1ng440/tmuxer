// MIT License

// Copyright (c) 2023 Asaduzzaman Pavel <contact@iampavel.dev>

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type Project struct {
	Name     string
	FullPath string
	HomePath string
}

type Config struct {
	ProjectBase []string `yaml:"base"`
}

func (cfg *Config) NormalizePaths() error {
	for i := range cfg.ProjectBase {
		p, err := normalizePath(cfg.ProjectBase[i])
		if err != nil {
			return err
		}
		cfg.ProjectBase[i] = p
	}

	return nil
}

const defaultConfigPath = "~/.config/tmux/tmuxer.yaml"

var (
	projectBase = pflag.StringSliceP(
		"base",
		"b",
		[]string{},
		"Base directories where projects are located",
	)
	projectMarkers = pflag.StringSliceP(
		"marker",
		"m",
		[]string{".git"},
		"Directories containing any of these patterns will be considered as projects. Default: .git",
	)
	ignorePatterns = pflag.StringSliceP(
		"ignore",
		"i",
		[]string{},
		"Patterns to ignore projects",
	)
	configPath = pflag.StringP(
		"config",
		"c",
		defaultConfigPath,
		"Path to the configuration file",
	)
)

func main() {
	var (
		config = &Config{}
		err    error
	)
	pflag.Parse()

	cfgPath, err := normalizePath(*configPath)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	config, err = loadConfig(cfgPath)
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	if err := mergeFlagsWithConfig(config); err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}

	// Add ~ as default root if none provided
	if len(config.ProjectBase) == 0 {
		fmt.Println("Error: No project base path provided")
		os.Exit(1)
	}

	if err := config.NormalizePaths(); err != nil {
		fmt.Printf("Error: Failed to normalize config path: %s", err.Error())
		os.Exit(1)
	}

	projects, err := findProjectDirectories(config)
	if err != nil {
		fmt.Printf("Error: Failed to normalize config path: %s", err.Error())
		os.Exit(1)
	}

	projectDir, err := selectProjectDirectory(projects)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	err = startOrAttachToTmux(projectDir)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func loadConfig(configPath string) (*Config, error) {
	config := &Config{}

	if configPath == "" || configPath == "-" {
		return config, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func mergeFlagsWithConfig(config *Config) error {
	if len(*projectBase) > 0 {
		config.ProjectBase = append(config.ProjectBase, *projectBase...)
	}
	return nil
}

func findProjectDirectories(cfg *Config) ([]*Project, error) {
	ret := make(map[string]*Project)
	regex := regexp.MustCompile(`(\*|\*\*|\?|\[.*\]|\{[^}]*\})`)

	for _, basePattern := range cfg.ProjectBase {
		base, pattern := doublestar.SplitPattern(basePattern)
		patternUsed := len(regex.FindStringIndex(path.Base(pattern))) > 0
		homedir, _ := os.UserHomeDir()

		doublestar.GlobWalk(os.DirFS(base), pattern, func(p string, _ fs.DirEntry) error {
			if patternUsed {
				fullpath := path.Join(base, path.Dir(p))
				rel, err := filepath.Rel(homedir, fullpath)
				if err != nil {
					return err
				}

				project := &Project{
					Name:     path.Dir(p),
					FullPath: fullpath,
					HomePath: rel,
				}
				ret[project.FullPath] = project
			}
			_ = ret
			return nil
		})
	}

	// let's convert it to string slice.
	res := make([]*Project, len(ret))
	i := 0
	for _, v := range ret {
		res[i] = v
		i++
	}
	return res, nil
}

func selectProjectDirectory(projects []*Project) (*Project, error) {
	idx, err := fuzzyfinder.Find(
		projects,
		func(i int) string {
			return projects[i].Name
		},
		fuzzyfinder.WithPreviewWindow(func(i, _, _ int) string {
			if i == -1 {
				return ""
			}
			return fmt.Sprintf(
				"Name: %s\nFull Path: %s",
				projects[i].Name,
				projects[i].FullPath,
			)
		}))
	if err != nil {
		return nil, err
	}

	fmt.Printf("Starting selected project: %s\n", projects[idx].Name)
	return projects[idx], nil
}

func startOrAttachToTmux(project *Project) error {
	sessionExists := false
	inTmux := os.Getenv("TMUX") != ""

	cmd := exec.Command("tmux", "list-sessions")
	output, err := cmd.CombinedOutput()
	fmt.Println(strings.Contains(string(output), "no server running"))
	if err != nil && !strings.Contains(string(output), "no server running") {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, project.Name) {
			sessionExists = true
			break
		}
	}

	resetTerminal()

	switch {
	case sessionExists && inTmux:
		fmt.Printf("tmux switch-client -t %s", project.Name)
		return nil
	case sessionExists:
		fmt.Printf("tmux attach-session -t %s", project.Name)
		return nil
	default:
		if err := runTmuxCommand("new-session", "-d", "-s", project.Name, "-c", project.FullPath); err != nil {
			return err
		}
		return startOrAttachToTmux(project)
	}
}

func runTmuxCommand(cmdName string, args ...string) error {
	targ := append([]string{"tmux", cmdName}, args...)
	cmd := exec.Command("sh", "-c", strings.Join(targ, " "))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resetTerminal() {
	cmd := exec.Command("reset")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func normalizePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[1:])
	}
	if strings.HasPrefix(path, "$HOME") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[5:])
	}
	return filepath.Abs(path)
}
