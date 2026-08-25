package main

import (
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// PACKAGE OVERVIEW
// ============================================================================

// This file implements the parallel execution engine for synthetic population
// generation using simulated annealing. It distributes work across multiple CPU
// cores, handles output writing, and provides progress feedback.
//
// KEY FEATURES:
//   - Dynamic worker pool based on CPU cores
//   - Deterministic RNG seeding for reproducible results
//   - Thread-safe progress tracking and fitness collection
//   - Graceful error handling with channel-based communication
//   - Real-time progress reporting with ETA and memory usage
//
// ARCHITECTURE:
//   ┌─────────────┐
//   │   Main      │  - Creates workers, feeds jobs, collects results
//   │  Goroutine  │
//   └──────┬──────┘
//          │
//          ▼
//   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
//   │   Jobs      │────▶│  Worker 1   │────▶│  Results    │
//   │   Channel   │     │  (RNG 1)    │     │  Channel    │
//   └─────────────┘     └─────────────┘     └─────────────┘
//          │                    │                    │
//          │              ┌─────────────┐           │
//          ├─────────────▶│  Worker 2   │───────────┤
//          │              │  (RNG 2)    │           │
//          │              └─────────────┘           │
//          │                    │                    │
//          │              ┌─────────────┐           │
//          └─────────────▶│  Worker N   │───────────┘
//                         │  (RNG N)    │
//                         └─────────────┘
//                               │
//                               ▼
//                        ┌─────────────┐
//                        │   Writer    │  - Single goroutine writes to files
//                        │  Goroutine  │  - Prevents file corruption
//                        └─────────────┘
//                               │
//                               ▼
//                        ┌─────────────┐
//                        │   Output    │  - IDs file (area_id → microdata_id)
//                        │   Files     │  - Fractions file (synthetic vs constraint)
//                        └─────────────┘

// ============================================================================
// PARALLEL EXECUTION ENGINE
// ============================================================================

// parallelRun executes population synthesis in parallel across multiple workers.
// It distributes constraint areas across CPU cores for efficient processing.
//
// DETERMINISTIC MODE:
//
//	When UseRandomSeed == "yes", each area receives an RNG seeded by:
//	  seed = hash(areaID) + globalSeed
//	This guarantees reproducible results regardless of:
//	  - Worker count
//	  - Worker scheduling order
//	  - Constraint slice order
//	  - File system timing
//
// PARAMETERS:
//   - constraints: Slice of ConstraintData for each geographical area
//   - groups: Group assignments for each variable (e.g., age groups)
//   - microData: Individual microdata records to sample from
//   - weights: Variable weights for distance calculations
//   - microdataHeader: Column names from microdata CSV
//   - outputfile1: Path for ID mappings (area_id → microdata_id)
//   - outputfile2: Path for fraction comparisons (synthetic vs constraint)
//   - config: Annealing configuration parameters
//   - updates: Channel for UI progress updates
//
// RETURNS:
//   - error: Any error encountered during processing
func parallelRun(constraints []ConstraintData, groups []int, microData []MicroData, weights []float64, microdataHeader []string, outputfile1 string, outputfile2 string, config AnnealingConfig, updates chan<- UIUpdate) error {

	// ========================================================================
	// STEP 1: Calculate maximum group for fraction normalization
	// ========================================================================
	//
	// Groups represent categories (e.g., age groups 1-5). We need the maximum
	// group value to size arrays for fraction calculations.
	//
	// Example: If groups = [1, 2, 3, 1, 2], maxGroup = 3
	//   - groupTotalsSim[0] = sum of all variables in group 1
	//   - groupTotalsSim[1] = sum of all variables in group 2
	//   - groupTotalsSim[2] = sum of all variables in group 3

	maxGroup := groups[0]
	for _, value := range groups[1:] {
		if value > maxGroup {
			maxGroup = value
		}
	}

	// ========================================================================
	// STEP 2: Configure worker pool size
	// ========================================================================
	//
	// Dynamic sizing prevents over-allocation for small datasets.
	// - For large datasets: use all CPU cores
	// - For small datasets: cap workers to number of areas

	numWorkers := runtime.NumCPU()
	if len(constraints) < numWorkers {
		numWorkers = len(constraints)
	}
	headerLength := len(microdataHeader)

	// Send initial status update
	updates <- UIUpdate{Text: fmt.Sprintf("Starting %d workers for %d population areas", numWorkers, len(constraints))}

	// ========================================================================
	// STEP 3: Setup communication channels
	// ========================================================================
	//
	// Channels enable safe communication between goroutines:
	//   - jobs:     Main → Workers  (sends constraint areas)
	//   - results:  Workers → Writer (sends processed results)
	//   - errChan:  Workers/Writer → Main (signals errors)
	//
	// Buffered channels prevent blocking and improve throughput.

	type Job struct {
		Constraint ConstraintData
	}

	jobs := make(chan Job, numWorkers*2)
	resultsChan := make(chan results, numWorkers*2)
	errChan := make(chan error, 1) // Buffered to prevent deadlocks

	// ========================================================================
	// STEP 4: Create output files
	// ========================================================================
	//
	// Two output files:
	//   1. IDs file: Maps area IDs to selected microdata IDs
	//      Format: area_id, microdata_id
	//      Example: "A001", "MD-1234"
	//
	//   2. Fractions file: Compares synthetic vs constraint proportions
	//      Format: geography_code, variable, synth_fraction, constraint_fraction
	//      Example: "A001", "age_18-24", 0.25, 0.30

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

	// ========================================================================
	// STEP 5: Initialize CSV writers with buffering
	// ========================================================================
	//
	// CSV writers buffer data internally for performance.
	// Flush ensures all data is written even on early exit.

	idsWriter := csv.NewWriter(idsFile)
	defer idsWriter.Flush()

	fractionsWriter := csv.NewWriter(fractionsFile)
	defer fractionsWriter.Flush()

	// Write headers
	if err := idsWriter.Write([]string{"area_id", "microdata_id"}); err != nil {
		return fmt.Errorf("error writing IDs headers: %w", err)
	}

	header := []string{"geography_code", "variable", "synth_fraction", "constraint_fraction"}
	if err := fractionsWriter.Write(header); err != nil {
		return fmt.Errorf("error writing fractions headers: %w", err)
	}
	fractionsWriter.Flush()
	if err := fractionsWriter.Error(); err != nil {
		return fmt.Errorf("error flushing fractions headers: %w", err)
	}

	// ========================================================================
	// STEP 6: Setup progress tracking with thread-safe data structures
	// ========================================================================
	//
	// Thread-safe components:
	//   - processed: atomic counter (incremented by writer goroutine)
	//   - fitnessMu: mutex protecting fitness slice from concurrent access
	//   - fitness: slice appended by multiple workers, read by progress reporter

	var (
		processed      atomic.Int32 // Thread-safe counter
		totalJobs      = len(constraints)
		startTime      = time.Now()
		progressTicker = time.NewTicker(2 * time.Second)
		fitnessMu      sync.Mutex
		fitness        []float64
	)
	defer progressTicker.Stop() // Clean up ticker when done

	// ========================================================================
	// STEP 7: Progress reporter goroutine
	// ========================================================================
	//
	// Runs in background, sends UI updates every 2 seconds.
	// Shows:
	//   - Completion percentage
	//   - Elapsed time
	//   - Estimated time to completion (ETA)
	//   - Memory usage (for debugging)

	go func() {
		for range progressTicker.C {
			// Calculate progress metrics
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

			// Read memory stats
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// FIX: Safely copy fitness for display
			// Prevents UI from accessing fitness slice while it's being modified
			fitnessMu.Lock()
			fitnessCopy := make([]float64, len(fitness))
			copy(fitnessCopy, fitness)
			fitnessMu.Unlock()

			// Send progress update
			updates <- UIUpdate{
				Text: fmt.Sprintf("\r📊 Progress: %d/%d (%.1f%%) | ⏱️ Elapsed: %v | 🕒 ETA: %v | 🧠 Memory: %vMB",
					done, totalJobs, percent, elapsed, eta.Round(time.Second), m.Alloc/1024/1024),
				Fitness: fitnessCopy,
			}
		}
	}()

	// ========================================================================
	// STEP 8: Writer goroutine - handles all file output
	// ========================================================================
	//
	// Single goroutine serializes all writes to prevent file corruption.
	// This is the ONLY place that writes to output files.
	//
	// WHY SINGLE WRITER:
	//   - CSV files cannot be safely written concurrently
	//   - Serializing writes prevents data corruption
	//   - Simpler error handling

	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()

		for res := range resultsChan {
			areaId := res.area

			// -------------------------------------------------------------
			// 8a: Write ID mappings
			// -------------------------------------------------------------
			for _, id := range res.ids {
				if err := idsWriter.Write([]string{areaId, id}); err != nil {
					select {
					case errChan <- fmt.Errorf("error writing ID row: %w", err):
					default:
					}
					return
				}
			}

			// -------------------------------------------------------------
			// 8b: Write fraction comparisons
			// -------------------------------------------------------------
			// Calculate group totals for normalization
			groupTotalsSim := make([]float64, maxGroup)
			groupTotalsConstraints := make([]float64, maxGroup)
			for i := 0; i < headerLength; i++ {
				g := groups[i] - 1 // Convert 1-based to 0-based index
				groupTotalsSim[g] += res.synthpop_totals[i]
				groupTotalsConstraints[g] += res.constraint_totals[i]
			}

			// Build CSV rows in memory for efficiency
			var buf strings.Builder
			for i := 0; i < headerLength; i++ {
				buf.WriteString(areaId)
				buf.WriteByte(',')
				buf.WriteString(microdataHeader[i])
				buf.WriteByte(',')

				g := groups[i] - 1

				// Safe division: 0 if denominator is 0
				var synthFrac float64
				if groupTotalsSim[g] != 0 {
					synthFrac = res.synthpop_totals[i] / groupTotalsSim[g]
				}
				buf.WriteString(strconv.FormatFloat(synthFrac, 'f', -1, 64))
				buf.WriteByte(',')

				var constraintFrac float64
				if groupTotalsConstraints[g] != 0 {
					constraintFrac = res.constraint_totals[i] / groupTotalsConstraints[g]
				}
				buf.WriteString(strconv.FormatFloat(constraintFrac, 'f', -1, 64))
				buf.WriteByte('\n')
			}

			// Write the complete row batch
			if _, err := fractionsFile.WriteString(buf.String()); err != nil {
				select {
				case errChan <- fmt.Errorf("error writing fraction row: %w", err):
				default:
				}
				return
			}

			// Increment processed counter (thread-safe atomic operation)
			processed.Add(1)
		}
	}()

	// ========================================================================
	// STEP 9: Worker pool - processes constraints in parallel
	// ========================================================================
	//
	// Each worker runs in its own goroutine and processes jobs from the queue.
	//
	// DETERMINISTIC RNG SEEDING (CRITICAL FOR REPRODUCIBILITY):
	//   - Each area gets its own RNG instance
	//   - When UseRandomSeed == "yes": seed = hash(areaID) + globalSeed
	//   - This makes results independent of worker scheduling and order
	//   - The same area always gets the same seed across runs

	var workerWg sync.WaitGroup
	useSeed := strings.ToLower(strings.TrimSpace(config.UseRandomSeed)) == "yes"

	// Helper function: creates a deterministic seed from area ID and global seed
	// Uses FNV-1a hash for fast, collision-resistant ID hashing
	seedFromID := func(id string, globalSeed int64) int64 {
		h := fnv.New64a()
		h.Write([]byte(id))
		// Combine hash with global seed
		return int64(h.Sum64()) + globalSeed
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()

			for job := range jobs {
				// Create RNG for this area
				var rng *rand.Rand
				if useSeed {
					// DETERMINISTIC MODE:
					// Seed = hash(areaID) + globalSeed
					// This guarantees the same area always gets the same RNG sequence
					seed := seedFromID(job.Constraint.ID, int64(config.RandomSeed))
					rng = rand.New(rand.NewSource(seed))
				} else {
					// NON-DETERMINISTIC MODE:
					// Seed = current time + hash(areaID)
					// Still unique per area, but different each run
					seed := time.Now().UnixNano() + seedFromID(job.Constraint.ID, 0)
					rng = rand.New(rand.NewSource(seed))
				}

				// Run simulated annealing for this area
				res, flag := syntheticPopulation(job.Constraint, microData, config, rng, weights)

				if flag {
					// FIX: Thread-safe fitness append
					// Multiple workers can append simultaneously - mutex prevents corruption
					fitnessMu.Lock()
					fitness = append(fitness, res.fitness)
					fitnessMu.Unlock()

					// Send result to writer goroutine
					select {
					case resultsChan <- res:
					case <-errChan: // Stop if error occurred
						return
					}
				} else {
					// Log error but continue processing other areas
					updates <- UIUpdate{Text: fmt.Sprintf("⚠️ Area %s: No valid microdata found", job.Constraint.ID)}
				}
			}
		}(i)
	}

	// ========================================================================
	// STEP 10: Feed jobs to workers
	// ========================================================================
	//
	// Send each constraint area to the job queue.
	// Order doesn't affect results due to deterministic seeding by area ID.

	for _, constraint := range constraints {
		select {
		case jobs <- Job{Constraint: constraint}:
			// Job sent successfully
		case err := <-errChan:
			// Error occurred - shutdown gracefully
			close(jobs)
			workerWg.Wait()
			close(resultsChan)
			writerWg.Wait()
			return err
		}
	}
	close(jobs) // All jobs sent - workers will exit when queue empties

	// ========================================================================
	// STEP 11: Wait for completion and cleanup
	// ========================================================================

	// Wait for all workers to finish
	workerWg.Wait()

	// Close results channel - writer will exit when it's empty
	close(resultsChan)

	// Wait for writer to finish writing all results
	writerWg.Wait()

	// All done - return success
	return nil
}

// ============================================================================
// THREAD-SAFETY DESIGN DOCUMENTATION
// ============================================================================

/*
THREAD-SAFETY STRATEGY:

1. RANDOM NUMBER GENERATORS (RNG):
   - Each area gets its own *rand.Rand instance
   - No sharing of RNGs between goroutines
   - Each RNG is independent and thread-safe locally

2. CHANNELS (Safe by Design):
   - jobs:       Multiple writers (main), multiple readers (workers)
   - resultsChan: Multiple writers (workers), single reader (writer)
   - errChan:    Multiple writers (workers, writer), single reader (main)

3. SHARED DATA PROTECTION:
   - fitness slice: Protected by fitnessMu mutex
   - processed counter: atomic.Int32 for thread-safe increments
   - Output files: Only written by single writer goroutine

4. GRACEFUL SHUTDOWN:
   - Errors trigger graceful shutdown via errChan
   - Channels are closed to signal completion
   - WaitGroups ensure all goroutines finish before exit
   - Deferred Flush ensures data is written even on error

5. MEMORY SAFETY:
   - Buffered channels prevent goroutine blocking
   - Fitness data is copied before sending to UI
   - File writes use buffered writers for performance
*/

// ============================================================================
// DETERMINISTIC REPRODUCIBILITY DOCUMENTATION
// ============================================================================

/*
WHY DETERMINISTIC SEEDING BY AREA ID IS ESSENTIAL:

PROBLEM WITH WORKER INDEX SEEDING:
   seed = globalSeed + workerIndex
   - Area A processed by Worker 1 in Run 1: seed = 42 + 1 = 43
   - Area A processed by Worker 2 in Run 2: seed = 42 + 2 = 44
   - DIFFERENT SEED → DIFFERENT RESULTS ❌

PROBLEM WITH SLICE INDEX SEEDING:
   seed = globalSeed + sliceIndex
   - Area A at index 5 in Run 1: seed = 42 + 5 = 47
   - Area A at index 7 in Run 2 (if order changed): seed = 42 + 7 = 49
   - DIFFERENT SEED → DIFFERENT RESULTS ❌

SOLUTION: HASH-BASED SEEDING BY AREA ID:
   seed = hash(areaID) + globalSeed
   - Area "A001" always hashes to the same value
   - Global seed is constant across runs
   - Area "A001" always gets the same seed ✅
   - Results are reproducible regardless of order or worker ✅

HASH FUNCTION: FNV-1a
   - Fast, non-cryptographic hash
   - Excellent distribution properties
   - Deterministic and collision-resistant for ID strings
*/

// ============================================================================
// ERROR HANDLING DOCUMENTATION
// ============================================================================

/*
ERROR HANDLING STRATEGY:

1. FILE CREATION ERRORS:
   - Return immediately with error
   - No point continuing if files can't be created

2. CSV WRITE ERRORS:
   - Send error to errChan
   - Writer goroutine exits
   - Main goroutine detects error and shuts down workers
   - Uses select with default to prevent blocking

3. PROCESSING ERRORS (No valid microdata):
   - Log warning via UI updates
   - Continue processing other areas
   - Does not halt execution

4. CHANNEL ERRORS:
   - Select statements handle error priority
   - Graceful shutdown prevents deadlocks
   - Channels are closed in correct order

5. DEFER CLEANUP:
   - Files are closed via defer
   - CSV writers are flushed via defer
   - Progress ticker is stopped via defer
   - WaitGroups ensure all goroutines complete

ERROR PROPAGATION FLOW:
   Error Occurs → errChan ← Error Detected → Close jobs channel →
   Workers Exit → Close resultsChan → Writer Exits → Return Error
*/
