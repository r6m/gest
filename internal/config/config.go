package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const fileName = "gest.yaml"

// Config is the MVP Gest CLI configuration.
type Config struct {
	Project  ProjectConfig
	Entry    string
	Router   RouterConfig
	Generate GenerateConfig
	Build    BuildConfig
}

type ProjectConfig struct {
	Name string
}

type RouterConfig struct {
	Adapter string
}

type GenerateConfig struct {
	Root    string
	OpenAPI bool
}

type BuildConfig struct {
	Output   string
	Entry    string
	Generate bool
	Test     bool
	Trimpath bool
}

// Defaults returns the configuration used when gest.yaml is missing.
func Defaults() Config {
	return Config{
		Entry: "./cmd/api",
		Router: RouterConfig{
			Adapter: "chi",
		},
		Generate: GenerateConfig{
			Root:    ".",
			OpenAPI: false,
		},
		Build: BuildConfig{
			Output:   "bin/app",
			Entry:    "./cmd/api",
			Generate: true,
			Test:     false,
			Trimpath: true,
		},
	}
}

// Load reads gest.yaml from dir, applies defaults, and allows missing config files.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	return parse(data, path)
}

func parse(data []byte, path string) (Config, error) {
	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg := Defaults()
	applyRaw(&cfg, raw)
	return cfg, nil
}

func applyRaw(cfg *Config, raw rawConfig) {
	if raw.Project != nil && raw.Project.Name != nil {
		cfg.Project.Name = *raw.Project.Name
		cfg.Build.Output = defaultBuildOutput(cfg.Project.Name)
	}
	if raw.Entry != nil {
		cfg.Entry = *raw.Entry
		cfg.Build.Entry = *raw.Entry
	}
	if raw.Router != nil && raw.Router.Adapter != nil {
		cfg.Router.Adapter = *raw.Router.Adapter
	}
	if raw.Generate != nil {
		if raw.Generate.Root != nil {
			cfg.Generate.Root = *raw.Generate.Root
		}
		if raw.Generate.OpenAPI != nil {
			cfg.Generate.OpenAPI = *raw.Generate.OpenAPI
		}
	}
	if raw.Build != nil {
		if raw.Build.Output != nil {
			cfg.Build.Output = *raw.Build.Output
		}
		if raw.Build.Entry != nil {
			cfg.Build.Entry = *raw.Build.Entry
		}
		if raw.Build.Generate != nil {
			cfg.Build.Generate = *raw.Build.Generate
		}
		if raw.Build.Test != nil {
			cfg.Build.Test = *raw.Build.Test
		}
		if raw.Build.Trimpath != nil {
			cfg.Build.Trimpath = *raw.Build.Trimpath
		}
	}
}

func defaultBuildOutput(projectName string) string {
	if projectName == "" {
		return "bin/app"
	}
	return filepath.ToSlash(filepath.Join("bin", projectName))
}

type rawConfig struct {
	Project  *rawProjectConfig  `yaml:"project"`
	Entry    *string            `yaml:"entry"`
	Router   *rawRouterConfig   `yaml:"router"`
	Generate *rawGenerateConfig `yaml:"generate"`
	Build    *rawBuildConfig    `yaml:"build"`
}

type rawProjectConfig struct {
	Name *string `yaml:"name"`
}

type rawRouterConfig struct {
	Adapter *string `yaml:"adapter"`
}

type rawGenerateConfig struct {
	Root    *string `yaml:"root"`
	OpenAPI *bool   `yaml:"openapi"`
}

type rawBuildConfig struct {
	Output   *string `yaml:"output"`
	Entry    *string `yaml:"entry"`
	Generate *bool   `yaml:"generate"`
	Test     *bool   `yaml:"test"`
	Trimpath *bool   `yaml:"trimpath"`
}
