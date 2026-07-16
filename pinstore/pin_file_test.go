package pinstore_test

import (
	"os"
	"path/filepath"
	"testing"

	"driftpin/core"
	"driftpin/internal/testutil"
	"driftpin/pinstore"
)

func TestFilePinStoreRoundTrip(t *testing.T) {
	shapes := []struct {
		name  string
		state pinstore.PinState
	}{
		{"empty", pinstore.PinState{}},
		{"one_spec", pinstore.PinState{
			Specs: []core.Spec{testutil.NewSpec("s1", "h1")},
		}},
		{"one_marker", pinstore.PinState{
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
		}},
		{"one_spec_one_marker_no_link", pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
		}},
		{"one_spec_one_marker_one_link", pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}},
		{"one_resolution", pinstore.PinState{
			Specs:           []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers:         []core.Marker{testutil.NewMarker("m1", "h1")},
			Links:           []core.Link{testutil.NewLink("s1", "m1")},
			ResolutionState: []core.ResolutionState{testutil.NewResolutionState("s1", "m1", "ch1", "ch2")},
		}},
		{"many_specs", pinstore.PinState{
			Specs: []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2"), testutil.NewSpec("s3", "h3")},
		}},
		{"many_markers", pinstore.PinState{
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2"), testutil.NewMarker("m3", "h3")},
		}},
		{"many_links_2x2", pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2")},
			Links:   []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2")},
		}},
		{"many_resolutions", pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2")},
			Links:   []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m2")},
			ResolutionState: []core.ResolutionState{
				testutil.NewResolutionState("s1", "m1", "ch1", "ch2"),
				testutil.NewResolutionState("s2", "m2", "ch3", "ch4"),
			},
		}},
		{"full_graph_3x3", pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1"), testutil.NewSpec("s2", "h2"), testutil.NewSpec("s3", "h3")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1"), testutil.NewMarker("m2", "h2"), testutil.NewMarker("m3", "h3")},
			Links: []core.Link{
				testutil.NewLink("s1", "m1"), testutil.NewLink("s1", "m2"), testutil.NewLink("s1", "m3"),
				testutil.NewLink("s2", "m1"), testutil.NewLink("s2", "m2"), testutil.NewLink("s2", "m3"),
				testutil.NewLink("s3", "m1"), testutil.NewLink("s3", "m2"), testutil.NewLink("s3", "m3"),
			},
			ResolutionState: []core.ResolutionState{
				testutil.NewResolutionState("s1", "m1", "ch1", "ch2"),
				testutil.NewResolutionState("s2", "m2", "ch3", "ch4"),
			},
		}},
		{"specs_with_locations", pinstore.PinState{
			Specs: []core.Spec{
				testutil.NewSpecWithLocation("s1", "h1", "/project/specs/auth.xml", 42),
				testutil.NewSpecWithLocation("s2", "h2", "/project/specs/api.xml", 88),
			},
			Markers: []core.Marker{
				testutil.NewMarkerWithLocation("m1", "h1", "/project/src/auth.go", 15),
				testutil.NewMarkerWithLocation("m2", "h2", "/project/src/api.go", 200),
			},
			Links: []core.Link{testutil.NewLink("s1", "m1"), testutil.NewLink("s2", "m2")},
		}},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			dir := t.TempDir()
			store := pinstore.NewFilePinStore(dir)

			err := store.Save(shape.state)
			testutil.AssertNoError(t, err)

			loaded, err := store.Load()
			testutil.AssertNoError(t, err)
			testutil.AssertPinStateEquals(t, loaded, shape.state)
		})
	}
}

func TestFilePinStoreLoadMissing(t *testing.T) {
	t.Run("missing_file_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		store := pinstore.NewFilePinStore(dir)
		_, err := store.Load()
		testutil.AssertErrorWraps(t, err, pinstore.ErrPinNotFound)
	})
}

func TestFilePinStoreLoadMalformed(t *testing.T) {
	t.Run("malformed_xml_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".driftpin"), 0755); err != nil {
			t.Fatal(err)
		}
		pinPath := filepath.Join(dir, ".driftpin", "state.xml")
		os.WriteFile(pinPath, []byte("not valid xml <"), 0644)
		store := pinstore.NewFilePinStore(dir)
		_, err := store.Load()
		if err == nil {
			t.Fatalf("expected error loading malformed XML")
		}
	})
}

func TestFilePinStoreSaveOverwrite(t *testing.T) {
	t.Run("save_overwrites_existing", func(t *testing.T) {
		dir := t.TempDir()
		store := pinstore.NewFilePinStore(dir)

		initial := pinstore.PinState{
			Specs:   []core.Spec{testutil.NewSpec("s1", "h1")},
			Markers: []core.Marker{testutil.NewMarker("m1", "h1")},
			Links:   []core.Link{testutil.NewLink("s1", "m1")},
		}
		err := store.Save(initial)
		testutil.AssertNoError(t, err)

		err = store.Save(pinstore.PinState{})
		testutil.AssertNoError(t, err)

		loaded, err := store.Load()
		testutil.AssertNoError(t, err)
		testutil.AssertPinStateEquals(t, loaded, pinstore.PinState{})
	})
}

func TestFilePinStoreSaveEmptyCreatesFile(t *testing.T) {
	t.Run("save_empty_creates_file", func(t *testing.T) {
		dir := t.TempDir()
		store := pinstore.NewFilePinStore(dir)

		err := store.Save(pinstore.PinState{})
		testutil.AssertNoError(t, err)

		pinPath := filepath.Join(dir, ".driftpin", "state.xml")
		if _, err := os.Stat(pinPath); os.IsNotExist(err) {
			t.Fatalf(".driftpin/state.xml not created")
		}
		baselinesDir := filepath.Join(dir, ".driftpin", "baselines")
		if info, err := os.Stat(baselinesDir); err != nil || !info.IsDir() {
			t.Fatalf(".driftpin/baselines/ not created")
		}
	})
}
