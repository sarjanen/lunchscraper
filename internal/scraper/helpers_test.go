package scraper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// WriteSingleJSON
// ---------------------------------------------------------------------------

func TestWriteSingleJSON_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	menu := RestaurantMenu{
		Restaurant: "Test Place",
		Location:   "Linköping",
		MenuType:   "daily",
		Week:       "2026W10",
		Items: []MenuItem{
			{Date: "2026-03-02", Name: "Pasta", Description: "Med pesto"},
		},
		Source: "https://example.com",
	}

	WriteSingleJSON(menu, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	var got RestaurantMenu
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got.Restaurant != "Test Place" {
		t.Errorf("got restaurant %q, want %q", got.Restaurant, "Test Place")
	}
	if len(got.Items) != 1 {
		t.Errorf("got %d items, want 1", len(got.Items))
	}
	if got.Items[0].Name != "Pasta" {
		t.Errorf("got item name %q, want %q", got.Items[0].Name, "Pasta")
	}
}

func TestWriteSingleJSON_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "test.json")

	WriteSingleJSON(RestaurantMenu{Restaurant: "Nested"}, path)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected nested file to exist: %v", err)
	}
}

func TestWriteSingleJSON_PrettyPrints(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	WriteSingleJSON(RestaurantMenu{Restaurant: "Pretty"}, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !json.Valid(data) {
		t.Error("expected valid JSON")
	}
	// Pretty-printed JSON contains newlines
	content := string(data)
	if content[0] != '{' {
		t.Error("expected JSON to start with '{'")
	}
	if len(content) < 20 {
		t.Error("expected pretty-printed (multi-line) JSON")
	}
}

// ---------------------------------------------------------------------------
// MergeJSON
// ---------------------------------------------------------------------------

func TestMergeJSON_CombinesFiles(t *testing.T) {
	dir := t.TempDir()

	restaurants := []RestaurantMenu{
		{Restaurant: "Place A", Location: "Here", Items: []MenuItem{{Name: "Dish A"}}},
		{Restaurant: "Place B", Location: "There", Items: []MenuItem{{Name: "Dish B1"}, {Name: "Dish B2"}}},
	}
	names := []string{"place_a.json", "place_b.json"}

	for i, r := range restaurants {
		data, _ := json.Marshal(r)
		os.WriteFile(filepath.Join(dir, names[i]), data, 0644)
	}

	outputPath := filepath.Join(dir, "lunches.json")
	if err := MergeJSON(dir, outputPath); err != nil {
		t.Fatalf("MergeJSON failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	var output Output
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("invalid output JSON: %v", err)
	}

	if output.GeneratedAt == "" {
		t.Error("expected generated_at to be set")
	}
	if len(output.Restaurants) != 2 {
		t.Fatalf("got %d restaurants, want 2", len(output.Restaurants))
	}

	found := map[string]bool{}
	for _, r := range output.Restaurants {
		found[r.Restaurant] = true
	}
	if !found["Place A"] || !found["Place B"] {
		t.Errorf("expected Place A and Place B, got %v", found)
	}
}

func TestMergeJSON_SkipsLunchesJSON(t *testing.T) {
	dir := t.TempDir()

	data, _ := json.Marshal(RestaurantMenu{Restaurant: "Real", Items: []MenuItem{{Name: "Dish"}}})
	os.WriteFile(filepath.Join(dir, "real.json"), data, 0644)

	oldOutput, _ := json.Marshal(Output{GeneratedAt: "old", Restaurants: []RestaurantMenu{{Restaurant: "Stale"}}})
	os.WriteFile(filepath.Join(dir, "lunches.json"), oldOutput, 0644)

	outputPath := filepath.Join(dir, "lunches.json")
	if err := MergeJSON(dir, outputPath); err != nil {
		t.Fatalf("MergeJSON failed: %v", err)
	}

	outData, _ := os.ReadFile(outputPath)
	var output Output
	json.Unmarshal(outData, &output)

	if len(output.Restaurants) != 1 {
		t.Fatalf("got %d restaurants, want 1 (lunches.json should be excluded)", len(output.Restaurants))
	}
	if output.Restaurants[0].Restaurant != "Real" {
		t.Errorf("got %q, want %q", output.Restaurants[0].Restaurant, "Real")
	}
}

func TestMergeJSON_SkipsInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	data, _ := json.Marshal(RestaurantMenu{Restaurant: "Good", Items: []MenuItem{{Name: "Dish"}}})
	os.WriteFile(filepath.Join(dir, "good.json"), data, 0644)

	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{invalid json"), 0644)

	outputPath := filepath.Join(dir, "lunches.json")
	if err := MergeJSON(dir, outputPath); err != nil {
		t.Fatalf("MergeJSON should succeed despite invalid file: %v", err)
	}

	outData, _ := os.ReadFile(outputPath)
	var output Output
	json.Unmarshal(outData, &output)

	if len(output.Restaurants) != 1 {
		t.Errorf("got %d restaurants, want 1 (bad file skipped)", len(output.Restaurants))
	}
}

func TestMergeJSON_SkipsEmptyRestaurantName(t *testing.T) {
	dir := t.TempDir()

	data, _ := json.Marshal(RestaurantMenu{Restaurant: "", Items: []MenuItem{{Name: "Dish"}}})
	os.WriteFile(filepath.Join(dir, "empty.json"), data, 0644)

	data2, _ := json.Marshal(RestaurantMenu{Restaurant: "Named", Items: []MenuItem{{Name: "Dish"}}})
	os.WriteFile(filepath.Join(dir, "named.json"), data2, 0644)

	outputPath := filepath.Join(dir, "lunches.json")
	if err := MergeJSON(dir, outputPath); err != nil {
		t.Fatal(err)
	}

	outData, _ := os.ReadFile(outputPath)
	var output Output
	json.Unmarshal(outData, &output)

	if len(output.Restaurants) != 1 {
		t.Errorf("got %d restaurants, want 1", len(output.Restaurants))
	}
}

func TestMergeJSON_ErrorsOnEmptyDir(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "lunches.json")

	if err := MergeJSON(dir, outputPath); err == nil {
		t.Error("expected error for empty directory, got nil")
	}
}

func TestMergeJSON_ErrorsOnMissingDir(t *testing.T) {
	if err := MergeJSON("/nonexistent/path", "/tmp/out.json"); err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestMergeJSON_IgnoresNonJSONFiles(t *testing.T) {
	dir := t.TempDir()

	data, _ := json.Marshal(RestaurantMenu{Restaurant: "Real"})
	os.WriteFile(filepath.Join(dir, "real.json"), data, 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0644)
	os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644)

	outputPath := filepath.Join(dir, "lunches.json")
	if err := MergeJSON(dir, outputPath); err != nil {
		t.Fatal(err)
	}

	outData, _ := os.ReadFile(outputPath)
	var output Output
	json.Unmarshal(outData, &output)

	if len(output.Restaurants) != 1 {
		t.Errorf("got %d restaurants, want 1", len(output.Restaurants))
	}
}

// ---------------------------------------------------------------------------
// WriteJSON
// ---------------------------------------------------------------------------

func TestWriteJSON_CreatesValidOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	output := Output{
		GeneratedAt: "2026-03-02T08:00:00Z",
		Restaurants: []RestaurantMenu{
			{Restaurant: "A", Items: []MenuItem{{Name: "X"}}},
			{Restaurant: "B", Items: []MenuItem{{Name: "Y"}, {Name: "Z"}}},
		},
	}

	WriteJSON(output, path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var got Output
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got.GeneratedAt != "2026-03-02T08:00:00Z" {
		t.Errorf("got generated_at %q", got.GeneratedAt)
	}
	if len(got.Restaurants) != 2 {
		t.Errorf("got %d restaurants, want 2", len(got.Restaurants))
	}
}

// ---------------------------------------------------------------------------
// Existing helper functions
// ---------------------------------------------------------------------------

func TestCurrentISOWeek(t *testing.T) {
	week := currentISOWeek()
	if len(week) < 6 {
		t.Errorf("unexpected ISO week format: %q", week)
	}
	if week[4] != 'W' {
		t.Errorf("expected 'W' at position 4, got %q", week)
	}
}

func TestWeekdayToDate(t *testing.T) {
	days := []string{"MÅNDAG", "TISDAG", "ONSDAG", "TORSDAG", "FREDAG", "UNKNOWN"}
	for _, day := range days {
		got := weekdayToDate(day)
		if len(got) != 10 {
			t.Errorf("weekdayToDate(%q) = %q, want YYYY-MM-DD format", day, got)
		}
	}

	mon := weekdayToDate("MÅNDAG")
	tue := weekdayToDate("TISDAG")
	if mon >= tue {
		t.Errorf("expected Tuesday (%s) after Monday (%s)", tue, mon)
	}
}

func TestWeekdayDates_ReturnsMonToFri(t *testing.T) {
	dates := weekdayDates()
	if len(dates) != 5 {
		t.Fatalf("got %d dates, want 5", len(dates))
	}
	for i := 1; i < len(dates); i++ {
		if dates[i] <= dates[i-1] {
			t.Errorf("dates not sequential: %s <= %s", dates[i], dates[i-1])
		}
	}
}

func TestWeekDateRange(t *testing.T) {
	mon, fri := weekDateRange()
	if len(mon) != 10 || len(fri) != 10 {
		t.Errorf("unexpected format: mon=%q fri=%q", mon, fri)
	}
	if fri <= mon {
		t.Errorf("expected Friday (%s) after Monday (%s)", fri, mon)
	}
}

func TestContainsSwedishMonth(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"VECKA 10 MARS", true},
		{"2 FEBRUARI 2026", true},
		{"HELLO WORLD", false},
		{"JANUARI", true},
		{"januari", false},
		{"", false},
	}

	for _, tt := range tests {
		got := containsSwedishMonth(tt.input)
		if got != tt.want {
			t.Errorf("containsSwedishMonth(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractDishName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Raggmunk med stekt rimmat fläsk", "Raggmunk"},
		{"Pannkakor serveras med sylt", "Pannkakor"},
		{"Köttbullar samt lingon", "Köttbullar"},
		{"Pasta på tallrik", "Pasta"},
		{"Short", "Short"},
	}

	for _, tt := range tests {
		got := extractDishName(tt.input)
		if got != tt.want {
			t.Errorf("extractDishName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractDishName_LongLine(t *testing.T) {
	long := "This is a very long dish name that exceeds sixty characters and should be truncated somewhere"
	got := extractDishName(long)
	if len(got) > 60 {
		t.Errorf("expected truncation for long input, got %d chars", len(got))
	}
}

func TestExpandWeeklySpecials(t *testing.T) {
	items := []MenuItem{
		{Date: "2026-03-02", Name: "Pasta", Description: "Med pesto"},
		{Date: "", Name: "Veckans fisk", Description: "Lax"},
	}
	closedDates := map[string]bool{"2026-03-06": true}

	expanded := expandWeeklySpecials(items, closedDates)

	weeklyCount := 0
	for _, item := range expanded {
		if item.Name == "Veckans fisk" {
			weeklyCount++
			if item.Date == "2026-03-06" {
				t.Error("Veckans fisk should not appear on closed Friday")
			}
		}
	}

	if weeklyCount != 4 {
		t.Errorf("got %d weekly specials, want 4 (5 weekdays minus 1 closed)", weeklyCount)
	}
}

func TestExpandWeeklySpecials_PreservesClosedDays(t *testing.T) {
	items := []MenuItem{
		{Date: "2026-03-04", Name: "STÄNGT", Closed: true},
		{Date: "", Name: "Veckans vegetariska", Description: "Halloumi"},
	}
	closedDates := map[string]bool{}

	expanded := expandWeeklySpecials(items, closedDates)

	closedFound := false
	for _, item := range expanded {
		if item.Closed {
			closedFound = true
		}
	}
	if !closedFound {
		t.Error("expected closed day to be preserved")
	}
}
