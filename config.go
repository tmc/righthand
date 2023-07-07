package main

import (
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

var defaultConfig = RightHandConfig{
	LLMModel:     "gpt-4",
	WhisperModel: "base.en",
	Programs: []ProgramFewShotExamples{
		{
			Program: "iTerm2",
			Examples: []FewShotExample{
				{Input: "change to my home directory",
					Output: "cd ~"},
				{Input: "new tab",
					Output: "{Command}+t"},
				{Input: "Interactively rebase the last 3 commits",
					Output: "git rebase -i HEAD~3"},
				// horizontal split
				{Input: "split horizontally",
					Output: "{Command+Shift}+d"},
			},
		},
		{
			Program: "Google Chrome",
			Examples: []FewShotExample{
				// visit cnn in chrome:
				{Input: "Visit CNN.com and a new tab.",
					Output: "{Command}+t\nhttps://cnn.com{Enter}"},
			},
		},
	},
}

func configPath() string {
	ucd, _ := os.UserConfigDir()
	return filepath.Join(ucd, "righthand", "config.yaml")
}

// loadConfig loads the configuration file for RightHand as yaml
func loadConfig() (RightHandConfig, error) {
	var config RightHandConfig
	err := loadYaml(configPath(), &config)
	if err != nil {
		return defaultConfig, err
	}
	return config, nil
}

// saveConfig saves the configuration file for RightHand as yaml
func saveConfig(config RightHandConfig) error {
	return saveYaml(configPath(), config)
}

func loadYaml(path string, v *RightHandConfig) error {
	f, err := os.Open(path)
	// if not exists, write default config
	if os.IsNotExist(err) {
		*v = defaultConfig
		return saveYaml(path, v)
	}
	return yaml.NewDecoder(f).Decode(v)

}

func saveYaml(path string, v interface{}) error {
	// create directory if not exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return yaml.NewEncoder(f).Encode(v)

}

// RightHandConfig is the configuration file for RightHand.
type RightHandConfig struct {
	LLMModel     string                   `json:"llm_model"`
	WhisperModel string                   `json:"whisper_model"`
	Programs     []ProgramFewShotExamples `json:"programs"`

	DumpWAVFile bool
}

// ProgramFewShotExamples is a program with a list of few-shot examples.
type ProgramFewShotExamples struct {
	Program  string           `json:"program"`
	Examples []FewShotExample `json:"examples"`
}

// FewShotExample is a few-shot example.
type FewShotExample struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}
