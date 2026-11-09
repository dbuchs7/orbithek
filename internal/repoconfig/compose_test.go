package repoconfig

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestComposeTemplateMatchesTheReadme: the Compose template parses, runs
// the released image on /data, and names only variables the README
// describes, commented out or not, so the two do not drift apart.
func TestComposeTemplateMatchesTheReadme(t *testing.T) {
	data, err := os.ReadFile("../../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Services map[string]struct {
			Image   string            `yaml:"image"`
			Volumes []string          `yaml:"volumes"`
			Env     map[string]string `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("compose.yaml: %v", err)
	}
	s, ok := c.Services["orbithek"]
	if !ok || s.Image != "ghcr.io/dbuchs7/orbithek:latest" || len(s.Volumes) != 1 || !strings.HasSuffix(s.Volumes[0], ":/data") {
		t.Fatalf("compose.yaml: service %+v", s)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	vars := regexp.MustCompile(`\bOT_[A-Z0-9_]+\b`).FindAllString(string(data), -1)
	if len(vars) < 10 {
		t.Fatalf("compose.yaml names only %d variables", len(vars))
	}
	for _, v := range vars {
		if !strings.Contains(string(readme), "`"+v+"`") && !strings.Contains(string(readme), v+"=") {
			t.Errorf("compose.yaml names %s, which README.md does not describe", v)
		}
	}
}
