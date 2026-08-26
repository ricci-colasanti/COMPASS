package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

func ReadMicroDataCSV(filename string) ([]MicroData, []string, error) {
	// println(filename, "**************************")
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	header, err := reader.Read()
	// println(header, "*******************************")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	var data []MicroData
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			//log.Printf("Error reading row: %v", err)
			continue
		}

		// Parse row
		id := row[0]
		//println(id)
		//Purpose: Creates a slice to store the float values from the CSV row.
		values := make([]float64, len(row)-1)
		for i, v := range row[1:] {
			num, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Printf("Invalid integer in row %v: %v", row, err)
				values[i] = 0 // or handle error differently
				continue
			}
			values[i] = num
		}

		data = append(data, MicroData{ID: id, Values: values})
	} // Uses Record struct without importing
	return data, header[1:], nil
}
