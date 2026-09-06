package main

import (
	"testing"
)

func floatPtr(v float64) *float64 {
	return &v
}

func TestParseMetricValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue *float64
		wantUnit  string
		wantErr   bool
	}{
		{"unknown", "U", nil, "", false},
		{"plain integer", "42", floatPtr(42), "", false},
		{"plain float", "23.7", floatPtr(23.7), "", false},
		{"with unit percent", "23.7%", floatPtr(23.7), "%", false},
		{"with unit bytes", "13958643712B", floatPtr(13958643712), "B", false},
		{"with unit MB", "6144MB", floatPtr(6144), "MB", false},
		{"negative value", "-5c", floatPtr(-5), "c", false},
		{"empty", "", nil, "", true},
		{"unit only", "MB", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, unit, err := parseMetricValue(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if unit != tt.wantUnit {
				t.Errorf("unit = %q, want %q", unit, tt.wantUnit)
			}

			if tt.wantValue == nil {
				if value != nil {
					t.Errorf("value = %v, want nil", *value)
				}
				return
			}

			if value == nil {
				t.Fatalf("value = nil, want %v", *tt.wantValue)
			}

			if *value != *tt.wantValue {
				t.Errorf("value = %v, want %v", *value, *tt.wantValue)
			}
		})
	}
}

func TestParseMetric(t *testing.T) {
	t.Run("value only", func(t *testing.T) {
		metric, err := parseMetric("value=42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Name != "value" {
			t.Errorf("Name = %q, want %q", metric.Name, "value")
		}

		if metric.Value == nil || *metric.Value != 42 {
			t.Errorf("Value = %v, want 42", metric.Value)
		}

		if metric.Warning != "" || metric.Critical != "" {
			t.Errorf("expected empty warning/critical, got %q/%q", metric.Warning, metric.Critical)
		}
	})

	t.Run("full thresholds and range", func(t *testing.T) {
		metric, err := parseMetric("cpu_usage=23.7%;80;95;0;100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Name != "cpu_usage" {
			t.Errorf("Name = %q, want %q", metric.Name, "cpu_usage")
		}

		if metric.Value == nil || *metric.Value != 23.7 {
			t.Errorf("Value = %v, want 23.7", metric.Value)
		}

		if metric.Unit != "%" {
			t.Errorf("Unit = %q, want %q", metric.Unit, "%")
		}

		if metric.Warning != "80" || metric.Critical != "95" {
			t.Errorf("Warning/Critical = %q/%q, want 80/95", metric.Warning, metric.Critical)
		}

		if metric.Minimum == nil || *metric.Minimum != 0 {
			t.Errorf("Minimum = %v, want 0", metric.Minimum)
		}

		if metric.Maximum == nil || *metric.Maximum != 100 {
			t.Errorf("Maximum = %v, want 100", metric.Maximum)
		}
	})

	t.Run("missing min/max left nil", func(t *testing.T) {
		metric, err := parseMetric("load1=0.42;4;8;0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Minimum == nil || *metric.Minimum != 0 {
			t.Errorf("Minimum = %v, want 0", metric.Minimum)
		}

		if metric.Maximum != nil {
			t.Errorf("Maximum = %v, want nil", *metric.Maximum)
		}
	})

	t.Run("quoted name", func(t *testing.T) {
		metric, err := parseMetric("'root disk used'=52%;85;95;0;100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Name != "root disk used" {
			t.Errorf("Name = %q, want %q", metric.Name, "root disk used")
		}
	})

	t.Run("quoted name with escaped quote", func(t *testing.T) {
		metric, err := parseMetric("'disk''s used'=52%")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Name != "disk's used" {
			t.Errorf("Name = %q, want %q", metric.Name, "disk's used")
		}
	})

	t.Run("unterminated quoted name", func(t *testing.T) {
		_, err := parseMetric("'disk used=52%")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("unknown value", func(t *testing.T) {
		metric, err := parseMetric("temperature=U")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if metric.Value != nil {
			t.Errorf("Value = %v, want nil", *metric.Value)
		}
	})

	t.Run("missing equals", func(t *testing.T) {
		_, err := parseMetric("novalue")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := parseMetric("=42")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("too many fields", func(t *testing.T) {
		_, err := parseMetric("value=1;2;3;4;5;6")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("invalid minimum", func(t *testing.T) {
		_, err := parseMetric("value=1;2;3;notanumber")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("invalid maximum", func(t *testing.T) {
		_, err := parseMetric("value=1;2;3;0;notanumber")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})
}

func TestSplitPerfDataFields(t *testing.T) {
	t.Run("simple space separated", func(t *testing.T) {
		fields, err := splitPerfDataFields("value=42 other=1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{"value=42", "other=1"}
		if len(fields) != len(want) {
			t.Fatalf("got %v, want %v", fields, want)
		}
		for i := range want {
			if fields[i] != want[i] {
				t.Errorf("field[%d] = %q, want %q", i, fields[i], want[i])
			}
		}
	})

	t.Run("quoted names containing spaces are kept together", func(t *testing.T) {
		fields, err := splitPerfDataFields("'root disk used'=52%;85;95;0;100 'data disk used'=71%;85;95;0;100")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(fields) != 2 {
			t.Fatalf("got %d fields, want 2: %v", len(fields), fields)
		}

		if fields[0] != "'root disk used'=52%;85;95;0;100" {
			t.Errorf("field[0] = %q", fields[0])
		}
	})

	t.Run("unterminated quote is an error", func(t *testing.T) {
		_, err := splitPerfDataFields("'unterminated=42")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("extra whitespace collapses", func(t *testing.T) {
		fields, err := splitPerfDataFields("  value=1   other=2  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(fields) != 2 {
			t.Fatalf("got %v, want 2 fields", fields)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		fields, err := splitPerfDataFields("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fields != nil {
			t.Errorf("got %v, want nil", fields)
		}
	})
}

func TestParsePerfData(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		metrics, err := parsePerfData("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if metrics != nil {
			t.Errorf("got %v, want nil", metrics)
		}
	})

	t.Run("whitespace only returns nil", func(t *testing.T) {
		metrics, err := parsePerfData("   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if metrics != nil {
			t.Errorf("got %v, want nil", metrics)
		}
	})

	t.Run("multiple metrics", func(t *testing.T) {
		metrics, err := parsePerfData("cpu_usage=23.7%;80;95;0;100 memory_used=6144MB;7000;7800;0;8192 load1=0.42;4;8;0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(metrics) != 3 {
			t.Fatalf("got %d metrics, want 3", len(metrics))
		}

		if metrics[0].Name != "cpu_usage" || metrics[1].Name != "memory_used" || metrics[2].Name != "load1" {
			t.Errorf("unexpected metric names: %+v", metrics)
		}
	})

	t.Run("propagates parse errors", func(t *testing.T) {
		_, err := parsePerfData("value=notanumber")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})

	t.Run("propagates split errors", func(t *testing.T) {
		_, err := parsePerfData("'unterminated=42")
		if err == nil {
			t.Fatalf("expected error, got none")
		}
	})
}
