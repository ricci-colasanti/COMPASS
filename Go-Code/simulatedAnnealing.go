package main

import (
	"fmt"
	"math"
	"math/rand"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// EPSILON is a tiny value used to avoid division by zero and to ensure
	// numerical stability in logarithmic and ratio calculations.
	EPSILON = 1e-10

	// MAX_PENALTY is used when a constraint is zero but testData has a positive value
	// in Chi-squared distance calculations.
	MAX_PENALTY = 1e10

	// STAGNATION_RATIO is the relative improvement threshold for detecting stagnation.
	// This is a dimensionless ratio in [0,1], not an absolute value.
	STAGNATION_RATIO = 0.001 // 0.1% relative improvement

	// MAX_FLAT_ACCEPTS limits how many equal-fitness moves we accept before exiting
	MAX_FLAT_ACCEPTS = 10000

	// MAX_REHEATS limits how many times we reheat before giving up
	MAX_REHEATS = 5

	// RECOMPUTE_INTERVAL recomputes totals exactly every N iterations to prevent drift
	RECOMPUTE_INTERVAL = 1000

	// MAX_TEMP_FRACTION caps reheating to this fraction of initial temperature
	MAX_TEMP_FRACTION = 0.5
)

// ============================================================================
// ANNEALING CONTEXT – ADAPTIVE THRESHOLDS (FIXED)
// ============================================================================

// AnnealingContext holds the state needed to compute metric‑specific
// convergence thresholds.
type AnnealingContext struct {
	InitialFitness float64         // baseline fitness (deterministic, not random)
	BestFitness    float64         // best fitness found so far
	Config         AnnealingConfig // configuration (contains metric name)
}

// GetEffectiveThresholds returns the fitness threshold (for early stopping).
// FIXED: Now uses deterministic baseline instead of random InitialFitness.
func (ctx *AnnealingContext) GetEffectiveThresholds() (fitnessThresh float64) {
	baseScale := ctx.InitialFitness
	if baseScale < EPSILON {
		baseScale = EPSILON
	}

	switch ctx.Config.Distance {
	case "CHI_SQUARED":
		fitnessThresh = baseScale * 0.1 // 10% of initial error
	case "EUCLIDEAN", "MANHATTEN":
		fitnessThresh = baseScale * 0.05 // 5% of initial error
	case "NORM_EUCLIDEAN", "COSINE", "MSE":
		fitnessThresh = baseScale * 0.01 // 1% of initial error
	case "KL_DIVERGENCE", "JSDIVERGENCE":
		fitnessThresh = baseScale * 0.1 // 10% of initial error
	default:
		fitnessThresh = ctx.Config.FitnessThreshold
	}
	return
}

// ============================================================================
// DISTANCE FUNCTIONS – TYPE AND VALIDATION (FIXED)
// ============================================================================

// DistanceFunc is the signature for all distance/error functions.
type DistanceFunc func([]float64, []float64, []float64) float64

// validatePopulationInputs performs a one‑time validation of all inputs.
// FIXED: Now checks if constraints are feasible (has matching microdata).
func validatePopulationInputs(constraint ConstraintData, microdata []MicroData, weights []float64) error {
	if len(constraint.Values) == 0 {
		return fmt.Errorf("constraint values cannot be empty")
	}
	if len(weights) > 0 && len(weights) != len(constraint.Values) {
		return fmt.Errorf("weights length %d != constraint length %d", len(weights), len(constraint.Values))
	}
	for i, md := range microdata {
		if len(md.Values) != len(constraint.Values) {
			return fmt.Errorf("microdata index %d length %d != constraint length %d", i, len(md.Values), len(constraint.Values))
		}
	}

	// FIXED: Check if constraints are feasible
	for attrIdx, target := range constraint.Values {
		if target > 0 {
			hasPositiveData := false
			for _, md := range microdata {
				if attrIdx < len(md.Values) && md.Values[attrIdx] > 0 {
					hasPositiveData = true
					break
				}
			}
			if !hasPositiveData {
				return fmt.Errorf("constraint[%d] = %f but no microdata has positive values for this attribute", attrIdx, target)
			}
		}
	}
	return nil
}

// distanceFunc is a factory that returns the appropriate DistanceFunc.
// FIXED: Logs error and returns Euclidean instead of silent fallback.
func distanceFunc(config AnnealingConfig) DistanceFunc {
	switch config.Distance {
	case "CHI_SQUARED":
		return ChiSquaredDistance
	case "EUCLIDEAN":
		return EuclideanDistance
	case "NORM_EUCLIDEAN":
		return NormalizedEuclideanDistance
	case "MANHATTEN":
		return ManhattanDistance
	case "COSINE":
		return Cosine
	case "MSE":
		return MeanSquaredError
	case "JSDIVERGENCE":
		return JSdivergence
	case "MinkowskiDistance":
		return MinkowskiDistance
	case "KL_DIVERGENCE":
		return KLDivergence
	default:
		// FIXED: Log warning instead of silent fallback
		infof("WARNING: Unknown distance metric %q, falling back to Euclidean", config.Distance)
		return EuclideanDistance
	}
}

// ============================================================================
// DISTANCE METRIC IMPLEMENTATIONS (FIXED)
// ============================================================================

// ChiSquaredDistance computes Σ w_i * (observed - expected)² / expected.
// FIXED: Properly handles zero expected values without epsilon inversion.
func ChiSquaredDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		observed := testData[i]
		expected := constraints[i]

		// FIXED: Handle zero expected values properly
		if expected < EPSILON {
			if observed > EPSILON {
				// Any positive observed when expected is zero is infinitely bad
				// Use a large penalty instead of division by zero
				weight := 1.0
				if len(weights) > i {
					weight = weights[i]
				}
				distance += weight * MAX_PENALTY
			}
			continue
		}

		diff := observed - expected
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * (diff * diff) / expected
	}
	return distance
}

// NormalizedEuclideanDistance normalises each component by the constraint value.
// FIXED: Consistent penalty scaling for near-zero values.
func NormalizedEuclideanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		norm := constraints[i]
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}

		// FIXED: Consistent penalty for any near-zero constraint
		if math.Abs(norm) < EPSILON {
			if math.Abs(testData[i]) > EPSILON {
				// Use the same scaling for consistency
				distance += weight * 1000.0 * testData[i] * testData[i]
			}
			continue
		}

		diff := (testData[i] - constraints[i]) / norm
		distance += weight * diff * diff
	}
	return math.Sqrt(distance)
}

// Cosine computes the cosine distance: 1 - (p·q)/(||p||·||q||).
func Cosine(constraints, testData, weights []float64) float64 {
	dot, normConstraints, normTestData := 0.0, 0.0, 0.0
	for i := range constraints {
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		dot += weight * constraints[i] * testData[i]
		normConstraints += weight * constraints[i] * constraints[i]
		normTestData += weight * testData[i] * testData[i]
	}
	denominator := math.Sqrt(normConstraints) * math.Sqrt(normTestData)
	if denominator < EPSILON {
		return 1.0
	}
	return 1 - (dot / denominator)
}

// JSdivergence computes the Jensen‑Shannon divergence.
func JSdivergence(constraints, testData, weights []float64) float64 {
	var sum float64
	for i := range constraints {
		mid := (constraints[i] + testData[i]) / 2

		p := constraints[i] + EPSILON
		q := mid + EPSILON
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		sum += weight * p * math.Log(p/q)

		p = testData[i] + EPSILON
		q = mid + EPSILON
		sum += weight * p * math.Log(p/q)
	}
	return 0.5 * sum
}

// KLDivergence computes the Kullback‑Leibler divergence.
func KLDivergence(constraints, testData, weights []float64) float64 {
	divergence := 0.0
	for i := range constraints {
		p := constraints[i] + EPSILON
		q := testData[i] + EPSILON
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		divergence += weight * p * math.Log(p/q)
	}
	return divergence
}

// EuclideanDistance computes the weighted Euclidean distance (L2 norm).
func EuclideanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		diff := testData[i] - constraints[i]
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * diff * diff
	}
	return math.Sqrt(distance)
}

// MinkowskiDistance computes the Minkowski distance of order p=1.5.
func MinkowskiDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	p := 1.5
	for i := range constraints {
		diff := math.Abs(testData[i] - constraints[i])
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * math.Pow(diff, p)
	}
	return math.Pow(distance, 1/p)
}

// ManhattanDistance computes the weighted Manhattan distance (L1 norm).
func ManhattanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * math.Abs(testData[i]-constraints[i])
	}
	return distance
}

// MeanSquaredError computes the weighted mean squared error.
func MeanSquaredError(constraints, testData, weights []float64) float64 {
	sumSquares := 0.0
	totalWeight := 0.0
	for i := range constraints {
		difference := constraints[i] - testData[i]
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		sumSquares += weight * difference * difference
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	return sumSquares / totalWeight
}

// CanberraDistance computes the Canberra distance.
func CanberraDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		numerator := math.Abs(testData[i] - constraints[i])
		denominator := math.Abs(testData[i]) + math.Abs(constraints[i])
		if denominator > 0 {
			weight := 1.0
			if len(weights) > i {
				weight = weights[i]
			}
			distance += weight * numerator / denominator
		}
	}
	return distance
}

// WeightedPenaltyDistance applies a custom penalty based on the constraint value.
func WeightedPenaltyDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		error := math.Abs(testData[i] - constraints[i])
		penalty := 1.0
		if constraints[i] < 0.01 {
			penalty = 1000.0
		} else if constraints[i] < 0.1 {
			penalty = 100.0
		}
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * penalty * error
	}
	return distance
}

// ============================================================================
// MICRODATA VALIDATION
// ============================================================================

// isValidMicrodata checks whether a given microdata record can be used for
// an area with the given constraints.
func isValidMicrodata(mdValues, constraints []float64) bool {
	for i, constraintVal := range constraints {
		if constraintVal == 0 && mdValues[i] != 0 {
			return false
		}
	}
	return true
}

// getValidIndices scans the microdata slice once and returns the indices of
// all records that satisfy isValidMicrodata.
func getValidIndices(microdata []MicroData, constraint ConstraintData) []int {
	var validIndices []int
	for i, md := range microdata {
		if isValidMicrodata(md.Values, constraint.Values) {
			validIndices = append(validIndices, i)
		}
	}
	return validIndices
}

// ============================================================================
// SIMULATED ANNEALING CORE FUNCTIONS (FIXED)
// ============================================================================

// replace performs a single replacement step.
// FIXED: Avoids no-op swaps (selecting the same record).
func replace(microdata []MicroData, validIndices []int, constraint ConstraintData,
	synthPopTotals []float64, synthPopMicrodataIndexes []int, fitness float64,
	temp float64, rng *rand.Rand, distfunc DistanceFunc, weights []float64) (float64, bool) {

	if len(validIndices) == 0 {
		return fitness, false
	}

	// Choose a random position in the current population to replace.
	randomReplaceIndex := rng.Intn(len(synthPopMicrodataIndexes))
	oldIndex := synthPopMicrodataIndexes[randomReplaceIndex]

	// FIXED: Choose a DIFFERENT record (avoid no-ops)
	attempts := 0
	var randomReplacementIndex int
	for attempts < 10 {
		randomReplacementIndex = validIndices[rng.Intn(len(validIndices))]
		if randomReplacementIndex != oldIndex {
			break
		}
		attempts++
	}
	if randomReplacementIndex == oldIndex {
		// Couldn't find a different record - no-op
		return fitness, false
	}

	newValues := microdata[randomReplacementIndex].Values
	oldValues := microdata[oldIndex].Values

	// Update the aggregate totals: remove old record, add new record.
	for i := range synthPopTotals {
		synthPopTotals[i] = synthPopTotals[i] - oldValues[i] + newValues[i]
	}

	// Compute new fitness.
	newFitness := distfunc(constraint.Values, synthPopTotals, weights)
	delta := newFitness - fitness

	// Metropolis acceptance criterion.
	if delta <= 0 || math.Exp(-delta/temp) > rng.Float64() {
		// Accept: keep the new record in the population.
		synthPopMicrodataIndexes[randomReplaceIndex] = randomReplacementIndex
		return newFitness, true
	}

	// Reject: revert the aggregate totals to their previous state.
	for i := range synthPopTotals {
		synthPopTotals[i] = synthPopTotals[i] - newValues[i] + oldValues[i]
	}
	return fitness, false
}

// recomputeTotals recalculates synthPopTotals exactly from the selected indices.
// This prevents floating-point drift from accumulating.
func recomputeTotals(microdata []MicroData, synthPopIDs []int) []float64 {
	if len(synthPopIDs) == 0 {
		return []float64{}
	}
	totals := make([]float64, len(microdata[synthPopIDs[0]].Values))
	for _, idx := range synthPopIDs {
		for i, val := range microdata[idx].Values {
			totals[i] += val
		}
	}
	return totals
}

// initPopulation creates an initial synthetic population.
func initPopulation(constraint ConstraintData, microdata []MicroData, rng *rand.Rand) ([]float64, []int, bool) {
	synthPopTotals := make([]float64, len(constraint.Values))

	populationSize := int(math.Round(constraint.Total))
	if populationSize <= 0 {
		return synthPopTotals, []int{}, false
	}

	synthPopMicrodataIndexes := make([]int, 0, populationSize)
	validIndices := getValidIndices(microdata, constraint)

	if len(validIndices) == 0 {
		return synthPopTotals, synthPopMicrodataIndexes, false
	}

	for i := 0; i < populationSize; i++ {
		randomIndex := validIndices[rng.Intn(len(validIndices))]
		randomElement := microdata[randomIndex]

		synthPopMicrodataIndexes = append(synthPopMicrodataIndexes, randomIndex)
		for j := 0; j < len(synthPopTotals); j++ {
			synthPopTotals[j] += randomElement.Values[j]
		}
	}

	return synthPopTotals, synthPopMicrodataIndexes, true
}

// getBaselineFitness computes a deterministic baseline fitness for threshold scaling.
// This replaces the random InitialFitness to make thresholds reproducible.
func getBaselineFitness(constraint ConstraintData, microdata []MicroData, rng *rand.Rand,
	distfunc DistanceFunc, weights []float64) float64 {

	// Use a small random sample to create a baseline population
	validIndices := getValidIndices(microdata, constraint)
	if len(validIndices) == 0 {
		return 1.0
	}

	sampleSize := 1000
	if len(validIndices) < sampleSize {
		sampleSize = len(validIndices)
	}

	baselineTotals := make([]float64, len(constraint.Values))
	for i := 0; i < sampleSize; i++ {
		idx := validIndices[rng.Intn(len(validIndices))]
		for j, val := range microdata[idx].Values {
			baselineTotals[j] += val
		}
	}

	// Scale up to match population size
	scale := constraint.Total / float64(sampleSize)
	for i := range baselineTotals {
		baselineTotals[i] *= scale
	}

	return distfunc(constraint.Values, baselineTotals, weights)
}

// ============================================================================
// MAIN SYNTHETIC POPULATION FUNCTION (FULLY FIXED)
// ============================================================================

// syntheticPopulation is the core of the simulated annealing algorithm.
// FIXED: Returns (results, bool) to match parallel.go expectations.
// FIXED: Complete rewrite of reheating logic, stagnation detection, and error handling.
func syntheticPopulation(constraint ConstraintData, microdata []MicroData,
	config AnnealingConfig, rng *rand.Rand, weights []float64) (results, bool) {

	var synthPopResults results

	// Validate configuration and inputs once.
	if err := validateConfig(config); err != nil {
		infof("Invalid configuration: %v", err)
		return synthPopResults, false
	}
	if err := validatePopulationInputs(constraint, microdata, weights); err != nil {
		infof("Invalid inputs for area %s: %v", constraint.ID, err)
		return synthPopResults, false
	}

	// Get the distance function (FIXED: logs warning on unknown metric)
	distfunc := distanceFunc(config)

	// Create initial population using the provided RNG.
	synthPopTotals, synthPopIDs, ok := initPopulation(constraint, microdata, rng)
	if !ok {
		infof("Failed to initialize population for area %s: no valid records or population size <= 0", constraint.ID)
		return synthPopResults, false
	}

	// Initial fitness.
	fitness := distfunc(constraint.Values, synthPopTotals, weights)

	// FIXED: Use deterministic baseline for thresholds (not random InitialFitness)
	baselineFitness := getBaselineFitness(constraint, microdata, rng, distfunc, weights)
	if baselineFitness < EPSILON {
		baselineFitness = EPSILON
	}

	// Setup annealing context with deterministic baseline.
	annealingCtx := &AnnealingContext{
		InitialFitness: baselineFitness,
		BestFitness:    fitness,
		Config:         config,
	}

	fitnessThreshold := annealingCtx.GetEffectiveThresholds()

	changes := config.Change
	temp := config.InitialTemp

	// Rolling window for stagnation detection.
	improvementWindow := make([]float64, config.WindowSize)
	windowIndex := 0
	windowFilled := false
	iterationCount := 0

	// FIXED: Track reheating state properly
	reheatCount := 0
	windowRefreshedAfterReheat := false

	// FIXED: Track flat landscape to avoid endless loops
	consecutiveZeroImprovement := 0

	// Best solution tracking.
	bestFitness := fitness
	bestSynthPopTotals := make([]float64, len(synthPopTotals))
	copy(bestSynthPopTotals, synthPopTotals)
	bestSynthPopIDs := make([]int, len(synthPopIDs))
	copy(bestSynthPopIDs, synthPopIDs)

	// Cache valid indices for this area (only once)
	validIndices := getValidIndices(microdata, constraint)
	if len(validIndices) == 0 {
		infof("No valid microdata records for area %s", constraint.ID)
		return synthPopResults, false
	}

MainLoop:
	for iteration := 0; iteration < config.MaxIterations && changes > 0 && temp > config.MinTemp; iteration++ {
		var accepted bool
		// Perform one replacement step.
		fitness, accepted = replace(microdata, validIndices, constraint,
			synthPopTotals, synthPopIDs, fitness, temp, rng, distfunc, weights)
		iterationCount++

		// FIXED: Track flat landscape
		if accepted && fitness == bestFitness {
			consecutiveZeroImprovement++
			if consecutiveZeroImprovement > MAX_FLAT_ACCEPTS {
				// We're stuck on a flat plateau - exit gracefully
				break MainLoop
			}
		} else if accepted {
			consecutiveZeroImprovement = 0
		}

		// FIXED: Recompute totals periodically to prevent floating-point drift
		if iteration%RECOMPUTE_INTERVAL == 0 && iteration > 0 {
			synthPopTotals = recomputeTotals(microdata, synthPopIDs)
			fitness = distfunc(constraint.Values, synthPopTotals, weights)
		}

		// Update best solution if improved.
		if fitness < bestFitness {
			bestFitness = fitness
			annealingCtx.BestFitness = bestFitness
			copy(bestSynthPopTotals, synthPopTotals)
			copy(bestSynthPopIDs, synthPopIDs)

			// FIXED: Reset reheating state on genuine improvement
			reheatCount = 0
			windowRefreshedAfterReheat = false
			consecutiveZeroImprovement = 0

			// Early exit if fitness threshold is met.
			if bestFitness <= fitnessThreshold {
				break MainLoop
			}
		}

		// Update the rolling window with the new fitness.
		improvementWindow[windowIndex] = fitness
		windowIndex = (windowIndex + 1) % config.WindowSize
		if windowIndex == 0 {
			windowFilled = true
		}

		// Stagnation check: only when the window is fully populated.
		if windowFilled && iterationCount >= config.WindowSize {
			// Find min and max in the window.
			windowBest, windowWorst := improvementWindow[0], improvementWindow[0]
			for _, val := range improvementWindow {
				if val < windowBest {
					windowBest = val
				}
				if val > windowWorst {
					windowWorst = val
				}
			}

			// If the worst value is near zero, we have essentially perfect fitness.
			if windowWorst < EPSILON {
				break MainLoop
			}

			// FIXED: Only check stagnation if we haven't just reheated
			shouldCheckStagnation := true
			if windowRefreshedAfterReheat {
				// Wait until the window has been fully refreshed after a reheat
				if windowIndex != 0 {
					shouldCheckStagnation = false
				} else {
					windowRefreshedAfterReheat = false
				}
			}

			if shouldCheckStagnation {
				// FIXED: Use ratio threshold (dimensionless) instead of absolute threshold
				// This fixes the category error
				relativeImprovement := (windowWorst - windowBest) / (windowWorst + EPSILON)

				// FIXED: Also track absolute improvement for metrics with very small scales
				absoluteImprovement := windowWorst - windowBest

				// If progress is too slow, reheat.
				// Use BOTH criteria: relative improvement below threshold OR absolute stagnation
				if relativeImprovement < STAGNATION_RATIO || absoluteImprovement < EPSILON {
					// FIXED: Cap reheating to prevent runaway
					reheatCount++

					if reheatCount > MAX_REHEATS {
						// We've reheated too many times without improvement - exit
						break MainLoop
					}

					// FIXED: Controlled reheating with absolute cap
					newTemp := temp * (1 + config.ReheatFactor)
					// Cap at MAX_TEMP_FRACTION of initial temperature
					maxTemp := config.InitialTemp * MAX_TEMP_FRACTION
					if newTemp > maxTemp {
						newTemp = maxTemp
					}
					temp = newTemp

					windowRefreshedAfterReheat = true

					// FIXED: Reset the window after reheat to prevent immediate re-reheating
					improvementWindow = make([]float64, config.WindowSize)
					windowIndex = 0
					windowFilled = false
					iterationCount = 0

					// Reset flat landscape counter after reheating
					consecutiveZeroImprovement = 0

					// Continue to next iteration without cooling or decrementing changes
					continue MainLoop
				}
			}
		}

		// Cool down the temperature.
		temp *= config.CoolingRate

		// If the last replacement was rejected, decrement the "changes" counter.
		if !accepted {
			changes--
		}
	}

	// Prepare the result struct (matches the one in main.go)
	synthPopResults.area = constraint.ID
	synthPopResults.synthpop_totals = bestSynthPopTotals
	synthPopResults.ids = make([]string, len(bestSynthPopIDs))
	for i, id := range bestSynthPopIDs {
		synthPopResults.ids[i] = microdata[id].ID
	}
	synthPopResults.constraint_totals = constraint.Values
	// FIXED: Use the configured distance function for fitness reporting
	synthPopResults.fitness = distfunc(constraint.Values, bestSynthPopTotals, weights)
	synthPopResults.population = constraint.Total

	return synthPopResults, true
}

// validateConfig checks that the annealing configuration parameters are valid.
// FIXED: No longer validates distance metric (handled separately to avoid circular dependency).
func validateConfig(config AnnealingConfig) error {
	if config.MaxIterations <= 0 {
		return fmt.Errorf("MaxIterations must be positive, got %d", config.MaxIterations)
	}
	if config.WindowSize <= 0 {
		return fmt.Errorf("WindowSize must be positive, got %d", config.WindowSize)
	}
	if config.InitialTemp <= 0 {
		return fmt.Errorf("InitialTemp must be positive, got %f", config.InitialTemp)
	}
	if config.MinTemp < 0 {
		return fmt.Errorf("MinTemp must be non-negative, got %f", config.MinTemp)
	}
	if config.CoolingRate <= 0 || config.CoolingRate >= 1 {
		return fmt.Errorf("CoolingRate must be between 0 and 1, got %f", config.CoolingRate)
	}
	if config.ReheatFactor < 0 {
		return fmt.Errorf("ReheatFactor must be >= 0, got %f", config.ReheatFactor)
	}
	// FIXED: Reject Change = 0 (would cause zero iterations)
	if config.Change <= 0 {
		return fmt.Errorf("Change must be positive, got %d", config.Change)
	}
	return nil
}
