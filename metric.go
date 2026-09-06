package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Metric struct {
	Name     string
	Value    *float64
	Unit     string
	Warning  string
	Critical string
	Minimum  *float64
	Maximum  *float64
}

func parseMetric(data string) (Metric, error) {
	var metric Metric

	name, values, found := strings.Cut(data, "=")

	if !found {
		return metric, fmt.Errorf("invalid metric %q: missing '='", data)
	}

	if name == "" {
		return metric, fmt.Errorf("invalid metric %q: missing name", data)
	}

	if strings.HasPrefix(name, "'") {
		if !strings.HasSuffix(name, "'") || len(name) < 2 {
			return metric, fmt.Errorf("invalid quoted metric name %q", name)
		}

		name = name[1 : len(name)-1]
		name = strings.ReplaceAll(name, "''", "'")
	}

	metric.Name = name

	parts := strings.Split(values, ";")
	if len(parts) > 5 {
		return metric, fmt.Errorf("invalid metric %q: too many fields", data)
	}

	value, unit, err := parseMetricValue(parts[0])

	if err != nil {
		return metric, fmt.Errorf("invalid metric %q: %w", data, err)
	}

	metric.Value = value
	metric.Unit = unit

	if len(parts) > 1 {
		metric.Warning = parts[1]
	}

	if len(parts) > 2 {
		metric.Critical = parts[2]
	}

	if len(parts) > 3 && parts[3] != "" {
		minimum, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return metric, fmt.Errorf("invalid minimum %q", parts[3])
		}
		metric.Minimum = &minimum
	}

	if len(parts) > 4 && parts[4] != "" {
		maximum, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return metric, fmt.Errorf("invalid maximum %q", parts[4])
		}
		metric.Maximum = &maximum
	}

	return metric, nil
}

func parseMetricValue(value string) (*float64, string, error) {
	if value == "U" {
		return nil, "", nil
	}

	for i := len(value); i > 0; i-- {
		number, err := strconv.ParseFloat(value[:i], 64)
		if err == nil {
			return &number, value[i:], nil
		}
	}

	return nil, "", fmt.Errorf("invalid metric value %q", value)
}

func splitPerfDataFields(perfData string) ([]string, error) {
	var fields []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(perfData); i++ {
		ch := perfData[i]

		switch {
		case ch == '\'':
			if inQuote && i+1 < len(perfData) && perfData[i+1] == '\'' {
				current.WriteByte(ch)
				current.WriteByte(perfData[i+1])
				i++
				continue
			}

			inQuote = !inQuote
			current.WriteByte(ch)

		case unicode.IsSpace(rune(ch)) && !inQuote:
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if inQuote {
		return nil, fmt.Errorf("unterminated quoted metric name")
	}

	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields, nil
}

func parsePerfData(perfData string) ([]Metric, error) {
	if strings.TrimSpace(perfData) == "" {
		return nil, nil
	}

	fields, err := splitPerfDataFields(perfData)
	if err != nil {
		return nil, err
	}

	metrics := make([]Metric, 0, len(fields))

	for _, field := range fields {
		metric, err := parseMetric(field)

		if err != nil {
			return nil, err
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}
