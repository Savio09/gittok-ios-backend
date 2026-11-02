package crawler

import "gopkg.in/yaml.v3"

// This file holds the Dart/pubspec parsing the classifier relies on. The
// classification review (Slice 3d) showed the old substring match turning the
// word "flutter" in a pubspec comment into a Flutter SDK declaration; parsing
// the actual dependency tables closes that. A comment is not a dependency.

// pubspec is the slice of pubspec.yaml the classifier reads: the dependency
// tables, so a real Flutter SDK dependency can be told from an incidental
// mention of the word "flutter".
type pubspec struct {
	Name            string         `yaml:"name"`
	Dependencies    map[string]any `yaml:"dependencies"`
	DevDependencies map[string]any `yaml:"dev_dependencies"`
}

// pubspecFlutterTable returns the table ("dependencies" or "dev_dependencies")
// in which a pubspec.yaml declares the Flutter SDK, or "" when it does not —
// parsed from YAML, so a comment mentioning flutter never counts.
func pubspecFlutterTable(content string) string {
	var p pubspec
	if err := yaml.Unmarshal([]byte(content), &p); err != nil {
		return ""
	}
	if _, ok := p.Dependencies["flutter"]; ok {
		return "dependencies"
	}
	if _, ok := p.DevDependencies["flutter"]; ok {
		return "dev_dependencies"
	}
	return ""
}

// pubspecUsesFlutter reports whether a pubspec.yaml declares a Flutter SDK
// dependency in either table.
func pubspecUsesFlutter(content string) bool { return pubspecFlutterTable(content) != "" }
