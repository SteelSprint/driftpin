package statestore_test

import (
	"os"
	"path/filepath"
	"testing"

	"drift/core"
	"drift/internal/testutil"
	"drift/statestore"
)

func TestFileStateStoreRoundTrip(t *testing.T) {
	shapes := []struct {
		name  string
		state statestore.State
	}{
		{"empty", statestore.State{}},
		{"one_spec", statestore.State{
			Specs: []core.Spec{testutil.NewSpec("s1", "h1")},
		}},
		{"one_marker", statestore.State{
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
		}},
		{"one_spec_one_marker_no_link", statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
		}},
		{"one_spec_one_marker_one_link", statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
			Edges:   []core.Edge{testutil.NewLink("s1", "m1")},
		}},
		{"one_resolution", statestore.State{
			Specs:           []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers:         []core.Marker{testutil.NewMarker("m1", "h1")},
			Edges:           []core.Edge{testutil.NewLink("s1", "m1")},
			Resolutions: []core.EdgeResolution{testutil.NewResolutionState("s1", "m1", "ch1", "ch2")},
		}},
		{"many_specs", statestore.State{
			Specs: []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2"), testutil.NewSpec("s3", "h3")},
		}},
		{"many_markers", statestore.State{
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2"), testutil.NewMarker("m3", "h3")},
		}},
		{"many_links_2x2", statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2")},
			Edges:   []core.Edge{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2")},
		}},
		{"many_resolutions", statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2")},
			Edges:   []core.Edge{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m2")},
			Resolutions: []core.EdgeResolution{
				testutil.NewResolutionState("s1", "m1", "ch1", "ch2"),
				testutil.NewResolutionState("s2", "m2", "ch3", "ch4"),
			},
		}},
		{"full_graph_3x3", statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2"), testutil.NewSpec("s3", "h3")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2"), testutil.NewMarker("m3", "h3")},
			Edges: []core.Edge{
				testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s1", "m3"),
				testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2"), testutil.NewLink("s2", "m3"),
				testutil.NewLink("s3", "m1"), testutil.NewLink("s3", "m2"), testutil.NewLink("s3", "m3"),
			},
			Resolutions: []core.EdgeResolution{
				testutil.NewResolutionState("s1", "m1", "ch1", "ch2"),
				testutil.NewResolutionState("s2", "m2", "ch3", "ch4"),
			},
		}},
		{"specs_with_locations", statestore.State{
			Specs: []core.Spec{
				testutil.NewSpecWithLocation("s1", "h1", "/project/specs/auth.xml", 42),
				testutil.NewSpecWithLocation("s2", "h2", "/project/specs/api.xml", 88),
			},
			Markers: []core.Marker{
				testutil.NewMarkerWithLocation("m1", "h1", "/project/src/auth.go", 15),
				testutil.NewMarkerWithLocation("m2", "h2", "/project/src/api.go", 200),
			},
			Edges: []core.Edge{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m2")},
		}},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			dir := t.TempDir()
			store := statestore.NewFileStateStore(dir)

			err := store.Save(shape.state)
			testutil.AssertNoError(t, err)

			loaded, err := store.Load()
			testutil.AssertNoError(t, err)
			testutil.AssertStateEquals(t, loaded, shape.state)
		})
	}
}

func TestFileStateStoreLoadMissing(t *testing.T) {
	t.Run("missing_file_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		store := statestore.NewFileStateStore(dir)
		_, err := store.Load()
		testutil.AssertErrorWraps(t, err, statestore.ErrStateNotFound)
	})
}

func TestFileStateStoreLoadMalformed(t *testing.T) {
	t.Run("malformed_xml_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".drift"), 0755); err != nil {
			t.Fatal(err)
		}
		stateFilePath := filepath.Join(dir, ".drift", "state.xml")
		os.WriteFile(stateFilePath, []byte("not valid xml <"), 0644)
		store := statestore.NewFileStateStore(dir)
		_, err := store.Load()
		if err == nil {
			t.Fatalf("expected error loading malformed XML")
		}
	})
}

func TestFileStateStoreSaveOverwrite(t *testing.T) {
	t.Run("save_overwrites_existing", func(t *testing.T) {
		dir := t.TempDir()
		store := statestore.NewFileStateStore(dir)

		initial := statestore.State{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
			Edges:   []core.Edge{testutil.NewLink("s1", "m1")},
		}
		err := store.Save(initial)
		testutil.AssertNoError(t, err)

		err = store.Save(statestore.State{})
		testutil.AssertNoError(t, err)

		loaded, err := store.Load()
		testutil.AssertNoError(t, err)
		testutil.AssertStateEquals(t, loaded, statestore.State{})
	})
}

func TestFileStateStoreSaveEmptyCreatesFile(t *testing.T) {
	t.Run("save_empty_creates_file", func(t *testing.T) {
		dir := t.TempDir()
		store := statestore.NewFileStateStore(dir)

		err := store.Save(statestore.State{})
		testutil.AssertNoError(t, err)

		stateFilePath := filepath.Join(dir, ".drift", "state.xml")
		if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
			t.Fatalf(".drift/state.xml not created")
		}
		baselinesDir := filepath.Join(dir, ".drift", "baselines")
		if info, err := os.Stat(baselinesDir); err != nil || !info.IsDir() {
			t.Fatalf(".drift/baselines/ not created")
		}
	})
}
