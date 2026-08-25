package main

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func initializeRNG(config AnnealingConfig, numWorkers int) []*rand.Rand {
	workerRNGs := make([]*rand.Rand, numWorkers)

	var masterRNG *rand.Rand
	useSeed := strings.ToLower(strings.TrimSpace(config.UseRandomSeed)) == "yes"
	if useSeed {
		// Deterministic mode
		masterRNG = rand.New(rand.NewSource(int64(config.RandomSeed)))
	} else {
		// Production mode (non-deterministic)
		masterRNG = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Seed worker RNGs from master
	for i := range workerRNGs {
		workerRNGs[i] = rand.New(rand.NewSource(masterRNG.Int63()))
	}

	return workerRNGs
}

// parallelRun executes population synthesis in parallel across multiple workers.
// It takes constraint data, microdata, output file paths, and annealing configuration,
// then distributes the work across CPU cores and writes results to CSV files.
//
// Parameters:
//   - constraints: Slice of ConstraintData defining each geographical area's constraints
//   - microData: Slice of MicroData containing individual population records
//   - outputfile1: Path for output CSV mapping area IDs to synthetic population IDs
//   - outputfile2: Path for output CSV comparing synthetic vs constraint fractions
//   - config: AnnealingConfig with optimization parameters
//
// Returns:
//   - error: Any error encountered during processing
func parallelRun(constraints []ConstraintData, groups []int, microData []MicroData, weights []float64, microdataHeader []string, outputfile1 string, outputfile2 string, config AnnealingConfig, updates chan<- UIUpdate) error {
	// Dynamic worker count - use either CPU count or constraint count, whichever is smaller
	fitness := make([]float64, 0, len(microData))
	//fmt.Println(groups)
	maxGroup := groups[0]
	for _, value := range groups[1:] {
		if value > maxGroup {
			maxGroup = value
		}
	}

	numWorkers := runtime.NumCPU()
	if len(constraints) < numWorkers {
		numWorkers = len(constraints)
	}
	headerLength := len(microdataHeader)
	updates <- UIUpdate{Text: fmt.Sprintf("Starting %d workers for %d population areas", numWorkers, len(constraints))}

	// Initialize RNGs based on config
	workerRNGs := initializeRNG(config, numWorkers)

	// Setup communication channels:
	// - jobs: feeds constraints to workers
	// - resultsChan: collects processed results from workers
	// - errChan: receives any processing errors (buffered to prevent deadlocks)
	jobs := make(chan ConstraintData, numWorkers*2)
	resultsChan := make(chan results, numWorkers*2)
	errChan := make(chan error, 1)

	// Create output files for:
	// 1. ID mappings (area_id → synthetic population IDs)
	// 2. Fraction comparisons (synthetic vs constraint fractions by variable)
	idsFile, err := os.Create(outputfile1)
	if err != nil {
		return fmt.Errorf("cannot create IDs file: %w", err)
	}
	defer idsFile.Close()

	fractionsFile, err := os.Create(outputfile2)
	if err != nil {
		return fmt.Errorf("cannot create fractions file: %w", err)
	}
	defer fractionsFile.Close()

	// Initialize CSV writers with buffering
	idsWriter := csv.NewWriter(idsFile)
	defer idsWriter.Flush() // Ensure all data is written even if function exits early

	fractionsWriter := csv.NewWriter(fractionsFile)
	defer fractionsWriter.Flush()

	// Write CSV headers for both output files
	if err := idsWriter.Write([]string{"area_id", "microdata_id"}); err != nil {
		return fmt.Errorf("error writing IDs headers: %w", err)
	}
	header := []string{"geography_code", "variable", "synth_fraction", "constraint_fraction"}
	if err := fractionsWriter.Write(header); err != nil {
		return fmt.Errorf("error writing fractions headers: %w", err)
	}
	fractionsWriter.Flush() // This will write the line to file immediately
	if err := fractionsWriter.Error(); err != nil {
		return fmt.Errorf("error flushing fractions headers: %w", err)
	}
	// Progress tracking setup
	var (
		processed      atomic.Int32 // Thread-safe counter for completed jobs
		totalJobs      = len(constraints)
		startTime      = time.Now()                      // Capture start time for ETA calculation
		progressTicker = time.NewTicker(2 * time.Second) // Update progress every 2s
	)
	defer progressTicker.Stop()

	// Progress reporter goroutine - displays real-time statistics
	go func() {
		for range progressTicker.C {
			elapsed := time.Since(startTime).Round(time.Second)
			done := processed.Load()
			remaining := totalJobs - int(done)
			percent := float64(done) / float64(totalJobs) * 100

			// Calculate ETA based on current processing rate
			var eta time.Duration
			if done > 0 {
				perItem := elapsed / time.Duration(done)
				eta = time.Duration(remaining) * perItem
			}

			// Include memory usage in progress report
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			updates <- UIUpdate{Text: fmt.Sprintf("\r📊 Progress: %d/%d (%.1f%%) | ⏱️ Elapsed: %v | 🕒 ETA: %v | 🧠 Memory: %vMB", done, totalJobs, percent, elapsed, eta.Round(time.Second), m.Alloc/1024/1024), Fitness: fitness}
		}
	}()

	// Writer goroutine - handles all output file writing
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		for res := range resultsChan {
			areaId := res.area

			// Write ID mappings (using existing CSV writer)
			for _, id := range res.ids {
				if err := idsWriter.Write([]string{areaId, id}); err != nil {
					select {
					case errChan <- fmt.Errorf("error writing ID row: %w", err):
					default:
					}
					return
				}
			}

			// Build the unquoted CSV line
			var buf strings.Builder

			groupTotalsSim := make([]float64, maxGroup)
			groupTotalsConstraints := make([]float64, maxGroup)
			for i := 0; i < headerLength; i++ {
				g := groups[i] - 1
				groupTotalsSim[g] += res.synthpop_totals[i]
				groupTotalsConstraints[g] += res.constraint_totals[i]
			}
			for i := 0; i < headerLength; i++ {
				buf.WriteString(areaId)
				buf.WriteByte(',')
				buf.WriteString(microdataHeader[i])
				buf.WriteByte(',')

				g := groups[i] - 1

				// Safe division for synth fraction
				var synthFrac float64
				if groupTotalsSim[g] != 0 {
					synthFrac = res.synthpop_totals[i] / groupTotalsSim[g]
				} else {
					synthFrac = 0.0
					// Or consider: math.NaN() if you want to mark invalid divisions
				}
				buf.WriteString(strconv.FormatFloat(synthFrac, 'f', -1, 64))
				buf.WriteByte(',')

				// Safe division for constraint fraction
				var constraintFrac float64
				if groupTotalsConstraints[g] != 0 {
					constraintFrac = res.constraint_totals[i] / groupTotalsConstraints[g]
				} else {
					constraintFrac = 0.0
				}
				buf.WriteString(strconv.FormatFloat(constraintFrac, 'f', -1, 64))
				buf.WriteByte('\n')
			}

			// Write raw string directly to file
			if _, err := fractionsFile.WriteString(buf.String()); err != nil {
				select {
				case errChan <- fmt.Errorf("error writing fraction row: %w", err):
				default:
				}
				return
			}

			processed.Add(1)
		}
	}()

	// Worker pool - processes constraints in parallel
	var workerWg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()
			rng := workerRNGs[workerID]
			for constraint := range jobs {
				// Generate synthetic population for this constraint area
				res, flag := syntheticPopulation(constraint, microData, config, rng, weights)
				if flag {
					// Send result or abort if error occurred
					select {
					case resultsChan <- res:
					case <-errChan: // Channel closed means error occurred
						return
					}
					fitness = append(fitness, res.fitness)
				} else {
					println(constraint.ID)
				}
			}
		}(i)
	}

	// Feed jobs to workers with error checking
	for _, constraint := range constraints {
		select {
		case jobs <- constraint: // Send next job
		case err := <-errChan: // Handle any errors from writers
			close(jobs)        // Signal workers to stop
			workerWg.Wait()    // Wait for workers to finish
			close(resultsChan) // Close results channel
			writerWg.Wait()    // Wait for writer to finish
			return err         // Return the error
		}
	}
	close(jobs) // All jobs sent

	// Wait for completion
	workerWg.Wait()    // All workers finished
	close(resultsChan) // No more results coming
	writerWg.Wait()    // All results written

	// Final performance report
	// elapsed := time.Since(startTime).Round(time.Second)
	// fmt.Printf("\n✅ Completed %d populations in %v (avg %.2f/sec)\n",
	// 	totalJobs, elapsed, float64(totalJobs)/elapsed.Seconds())

	return nil
}
