package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const defaultCriticalReminderInterval = 10 * time.Minute

type Config struct {
	StartupSpread string         `yaml:"startup_spread"`
	Defaults      DefaultsConfig `yaml:"defaults"`
	Imports       []string       `yaml:"imports"`
	Actions       []ActionConfig `yaml:"actions"`
	Checks        []CheckConfig  `yaml:"checks"`
}

type CheckConfig struct {
	Name                     string             `yaml:"name"`
	Command                  string             `yaml:"command"`
	Args                     []string           `yaml:"args"`
	Timeout                  string             `yaml:"timeout"`
	Interval                 string             `yaml:"interval"`
	Attempts                 int                `yaml:"attempts"`
	CriticalReminderInterval string             `yaml:"critical_reminder_interval"`
	Actions                  CheckActionsConfig `yaml:"actions"`
}

type ActionConfig struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Timeout string   `yaml:"timeout"`
}

type CheckActionsConfig struct {
	OK       []string `yaml:"ok"`
	Warning  []string `yaml:"warning"`
	Critical []string `yaml:"critical"`
	Unknown  []string `yaml:"unknown"`
}

type DefaultsConfig struct {
	Timeout                  string `yaml:"timeout"`
	Interval                 string `yaml:"interval"`
	Attempts                 int    `yaml:"attempts"`
	CriticalReminderInterval string `yaml:"critical_reminder_interval"`
}

type ImportedConfig struct {
	Checks []CheckConfig `yaml:"checks"`
}

type RuntimeConfig struct {
	Checks        []Check
	Actions       map[string]Action
	StartupSpread time.Duration
}

func loadConfigFile(filename string, config any) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to open config %q: %w", filename, err)
	}

	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err := decoder.Decode(config); err != nil {
		return fmt.Errorf("unable to parse config %q: %w", filename, err)
	}

	return nil
}

func validateCheck(check Check) error {
	if check.Name == "" {
		return fmt.Errorf("check name is required")
	}

	if check.Command == "" {
		return fmt.Errorf("check command is required")
	}

	if check.Timeout <= 0 {
		return fmt.Errorf("check timeout must be greater than zero")
	}

	if check.Interval <= 0 {
		return fmt.Errorf("check interval must be greater than zero")
	}

	if check.Attempts <= 0 {
		return fmt.Errorf("check attempts must be greater than zero")
	}

	if check.Timeout >= check.Interval {
		return fmt.Errorf("check timeout must be smaller than interval")
	}

	if check.CriticalReminderInterval <= 0 {
		return fmt.Errorf("critical reminder interval must be greater than zero")
	}

	return nil
}

func validateAllChecks(checks []Check) error {
	names := make(map[string]bool)

	for _, check := range checks {
		if names[check.Name] {
			return fmt.Errorf("duplicate check name %q", check.Name)
		}
		names[check.Name] = true
	}

	return nil
}

func findConfigFile() (string, error) {
	configFiles := []string{
		filepath.Join("config", "greybeard.yml"),
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		configFiles = append(configFiles, filepath.Join(homeDir, ".greybeard", "greybeard.yml"))
	}

	configFiles = append(configFiles, "/etc/greybeard/greybeard.yml")

	for _, filename := range configFiles {
		info, err := os.Stat(filename)

		if err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("config path %q is not a regular file", filename)
			}

			return filename, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf(
				"unable to check config path %q: %w",
				filename,
				err,
			)
		}

	}

	return "", fmt.Errorf(
		"unable to find Greybeard configuration. Looked for: \n    %s\n\nSpecify a config file with -c or --config-file",
		strings.Join(configFiles, "\n "),
	)

}

func loadConfig(filename string) (RuntimeConfig, error) {
	var config Config

	if err := loadConfigFile(filename, &config); err != nil {
		return RuntimeConfig{}, err
	}

	if config.StartupSpread == "" {
		return RuntimeConfig{}, fmt.Errorf("startup_spread is required")
	}

	startupSpread, err := time.ParseDuration(config.StartupSpread)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf(
			"invalid startup_spread %q: %w",
			config.StartupSpread,
			err,
		)
	}

	if startupSpread < 0 {
		return RuntimeConfig{}, fmt.Errorf(
			"startup_spread must be zero or greater",
		)
	}

	if err := validateDefaults(config.Defaults); err != nil {
		return RuntimeConfig{}, fmt.Errorf("invalid defaults: %w", err)
	}

	checks, err := buildChecks(config.Checks, config.Defaults)
	if err != nil {
		return RuntimeConfig{}, err
	}

	actions, err := buildActions(config.Actions)
	if err != nil {
		return RuntimeConfig{}, err
	}

	configDir := filepath.Dir(filename)

	for _, importPath := range config.Imports {
		if !filepath.IsAbs(importPath) {
			importPath = filepath.Join(configDir, importPath)
		}

		files, err := resolveImport(importPath, filename)
		if err != nil {
			return RuntimeConfig{}, err
		}

		for _, importedFile := range files {
			importedChecks, err := loadImportedChecks(
				importedFile,
				config.Defaults,
			)
			if err != nil {
				return RuntimeConfig{}, err
			}

			checks = append(checks, importedChecks...)

		}
	}

	if len(checks) == 0 {
		return RuntimeConfig{}, fmt.Errorf("configuration file contains no checks")
	}

	if err := validateAllChecks(checks); err != nil {
		return RuntimeConfig{}, err
	}

	if err := validateActionReferences(checks, actions); err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		Checks:        checks,
		Actions:       actions,
		StartupSpread: startupSpread,
	}, nil
}

func buildChecks(checkConfigs []CheckConfig, defaults DefaultsConfig) ([]Check, error) {
	var checks []Check

	for _, checkConfig := range checkConfigs {

		timeoutString := checkConfig.Timeout
		if timeoutString == "" {
			timeoutString = defaults.Timeout
		}

		timeout, err := time.ParseDuration(timeoutString)
		if err != nil {
			return nil, fmt.Errorf(
				"check %q has invalid timeout %q: %w",
				checkConfig.Name,
				timeoutString,
				err,
			)
		}

		intervalString := checkConfig.Interval
		if intervalString == "" {
			intervalString = defaults.Interval
		}

		interval, err := time.ParseDuration(intervalString)
		if err != nil {
			return nil, fmt.Errorf(
				"check %q has invalid interval %q: %w",
				checkConfig.Name,
				intervalString,
				err,
			)
		}

		attempts := checkConfig.Attempts
		if attempts == 0 {
			attempts = defaults.Attempts
		}

		criticalReminderIntervalString := checkConfig.CriticalReminderInterval

		if criticalReminderIntervalString == "" {
			criticalReminderIntervalString = defaults.CriticalReminderInterval
		}

		criticalReminderInterval := defaultCriticalReminderInterval

		if criticalReminderIntervalString != "" {
			criticalReminderInterval, err = time.ParseDuration(
				criticalReminderIntervalString,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"check %q has invalid critical reminder interval %q: %w",
					checkConfig.Name,
					criticalReminderIntervalString,
					err,
				)
			}
		}

		check := Check{
			Name:                     checkConfig.Name,
			Command:                  checkConfig.Command,
			Args:                     checkConfig.Args,
			Timeout:                  timeout,
			Interval:                 interval,
			Attempts:                 attempts,
			CriticalReminderInterval: criticalReminderInterval,
			Actions: CheckActions{
				OK:       checkConfig.Actions.OK,
				Warning:  checkConfig.Actions.Warning,
				Critical: checkConfig.Actions.Critical,
				Unknown:  checkConfig.Actions.Unknown,
			},
		}

		if err := validateCheck(check); err != nil {
			return nil, fmt.Errorf("check %q: %w", checkConfig.Name, err)
		}

		checks = append(checks, check)
	}

	if err := validateAllChecks(checks); err != nil {
		return nil, err
	}

	return checks, nil

}

func buildActions(actionConfigs []ActionConfig) (map[string]Action, error) {
	actions := make(map[string]Action, len(actionConfigs))

	for _, actionConfig := range actionConfigs {
		if _, exists := actions[actionConfig.Name]; exists {
			return nil, fmt.Errorf(
				"duplicate action name %q",
				actionConfig.Name,
			)
		}

		if actionConfig.Timeout == "" {
			return nil, fmt.Errorf(
				"action %q: tiemout is required",
				actionConfig.Name,
			)
		}

		timeout, err := time.ParseDuration(actionConfig.Timeout)
		if err != nil {
			return nil, fmt.Errorf(
				"action %q has invalid timeout %q: %w",
				actionConfig.Name,
				actionConfig.Timeout,
				err,
			)
		}

		action := Action{
			Name:    actionConfig.Name,
			Command: actionConfig.Command,
			Args:    actionConfig.Args,
			Timeout: timeout,
		}

		if err := validateAction(action); err != nil {
			return nil, fmt.Errorf(
				"action %q: %w",
				actionConfig.Name,
				err,
			)
		}

		actions[action.Name] = action
	}
	return actions, nil
}

func loadImportedChecks(filename string, defaults DefaultsConfig) ([]Check, error) {

	var config ImportedConfig

	if err := loadConfigFile(filename, &config); err != nil {
		return nil, err
	}

	if len(config.Checks) == 0 {
		return nil, fmt.Errorf("config %q contains no checks", filename)
	}

	return buildChecks(config.Checks, defaults)
}

func resolveImport(path string, mainFile string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("Unable to access import %q: %w", path, err)
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config directory %q: %w", path, err)
	}

	var files []string

	mainFile, err = filepath.Abs(mainFile)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve main config path %q: %w", path, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))

		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		filename := filepath.Join(path, entry.Name())

		absoluteFilename, err := filepath.Abs(filename)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve config path %q: %w", filename, err)
		}

		if absoluteFilename == mainFile {
			continue
		}

		files = append(files, filename)
	}

	return files, nil
}

func validateDefaults(defaults DefaultsConfig) error {
	timeout, err := time.ParseDuration(defaults.Timeout)

	if err != nil {
		return fmt.Errorf("invalid default timeout %q: %w", defaults.Timeout, err)
	}

	if timeout <= 0 {
		return fmt.Errorf("default timeout must be greater than zero")
	}

	interval, err := time.ParseDuration(defaults.Interval)

	if err != nil {
		return fmt.Errorf("invalid default interval %q: %w", defaults.Interval, err)
	}

	if interval <= 0 {
		return fmt.Errorf("default interval must be greater than zero")
	}

	if defaults.Attempts <= 0 {
		return fmt.Errorf("default attempts must be greater than zero")
	}

	if timeout >= interval {
		return fmt.Errorf("timeout must be smaller than interval")
	}

	return nil
}

func validateAction(action Action) error {
	if action.Name == "" {
		return fmt.Errorf("action name is required")
	}

	if action.Command == "" {
		return fmt.Errorf("action command is required")
	}

	if action.Timeout <= 0 {
		return fmt.Errorf("action timeout must be greater than zero")
	}

	return nil
}

func validateActionReferences(
	checks []Check,
	actions map[string]Action,
) error {

	for _, check := range checks {
		states := map[string][]string{
			"ok":       check.Actions.OK,
			"warning":  check.Actions.Warning,
			"critical": check.Actions.Critical,
			"unknown":  check.Actions.Unknown,
		}

		for state, names := range states {
			seen := make(map[string]bool)

			for _, name := range names {
				if seen[name] {
					return fmt.Errorf(
						"check %q has duplicate %s action %q",
						check.Name,
						state,
						name,
					)
				}

				seen[name] = true

				if _, exists := actions[name]; !exists {
					return fmt.Errorf(
						"check %q references undefined %s action %q",
						check.Name,
						state,
						name,
					)
				}

			}

		}
	}

	return nil
}
