package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 3d: a Flutter SDK dependency is read from the parsed pubspec tables, so
// a comment mentioning flutter does not make a plain Dart library "mobile".

func TestPubspecUsesFlutter(t *testing.T) {
	flutterApp := "name: myapp\nenvironment:\n  sdk: '>=3.0.0'\ndependencies:\n  flutter:\n    sdk: flutter\n"
	if !pubspecUsesFlutter(flutterApp) {
		t.Errorf("a real Flutter SDK dependency was not detected")
	}

	commented := "name: math\n# No flutter dependency; ordinary Dart library.\nenvironment:\n  sdk: ^3.0.0\n"
	if pubspecUsesFlutter(commented) {
		t.Errorf("a commented mention of flutter was read as an SDK dependency")
	}
}

func TestDartClassification(t *testing.T) {
	// Flutter app -> mobile.
	flutter := InspectionFromSources("Dart",
		[]string{"pubspec.yaml", "lib/main.dart"},
		map[string]string{"pubspec.yaml": "name: myapp\ndependencies:\n  flutter:\n    sdk: flutter\n"},
	)
	if got := Classify(flutter); got.Kind != apimodel.KindMobile {
		t.Errorf("flutter app: Kind = %q, want %q", got.Kind, apimodel.KindMobile)
	}

	// Plain Dart library whose pubspec only mentions flutter in a comment -> not mobile.
	plain := InspectionFromSources("Dart",
		[]string{"pubspec.yaml", "lib/math.dart"},
		map[string]string{"pubspec.yaml": "name: math\n# No flutter dependency; ordinary Dart library.\nenvironment:\n  sdk: ^3.0.0\n"},
	)
	if got := Classify(plain); got.Kind == apimodel.KindMobile {
		t.Errorf("plain Dart library classified as mobile from a comment (got %q)", got.Kind)
	}
}
