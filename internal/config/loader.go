package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) ([]Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf(`open config file %q: %w`, path, err)
	}
	defer f.Close()

	var ans []Rule

	decoder := yaml.NewDecoder(f)
	for {
		var rule Rule
		if err := decoder.Decode(&rule); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode config file %q (rule #%d): %w", path, len(ans), err)
		}
		ans = append(ans, rule)
	}

	return ans, nil
}
