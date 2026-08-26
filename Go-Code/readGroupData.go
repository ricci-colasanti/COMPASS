package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

func ReadGroupingCSV(filename string) ([]int, []string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read and store the header for later use
	_, err = reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read header from %s: %w", filename, err)
	}

	var groups []int
	var headers []string

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading row: %v", err)
			continue
		}

		// Expecting exactly 2 columns: ID and value
		if len(row) != 2 {
			log.Printf("Skipping invalid row (expected 2 columns, got %d): %v", len(row), row)
			continue
		}

		// Parse the ID and value
		headers = append(headers, row[0])
		value, err := strconv.Atoi(row[1])
		if err != nil {
			log.Printf("Invalid number in row %v: %v", row, err)
			continue
		}
		groups = append(groups, value)

	}
	return groups, headers, nil
}
