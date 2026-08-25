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

	// These constants are symbolic identifiers for distance metric types.
	// They are not used directly in the code but serve as documentation.
	KL_DIVERGENCE = iota
	CHI_SQUARED
	EUCLIDEAN
	NORM_EUCLIDEAN
	MANHATTEN
)

// ============================================================================
// ANNEALING CONTEXT – ADAPTIVE THRESHOLDS
// ============================================================================

// AnnealingContext holds the state needed to compute metric‑specific
// convergence thresholds. Different distance metrics have different scales;
// using a fixed absolute threshold would be inappropriate. This struct
// allows us to adapt thresholds to the initial fitness and the chosen metric.
type AnnealingContext struct {
	InitialFitness float64         // fitness at the start of annealing
	BestFitness    float64         // best fitness found so far
	Config         AnnealingConfig // configuration (contains metric name)
}

// GetEffectiveThresholds returns the fitness threshold (for early stopping)
// and the improvement threshold (for stagnation detection) tailored to the
// distance metric being used.
//
// The logic scales the thresholds relative to the initial fitness:
//   - For large‑scale metrics (Euclidean, Manhattan) we aim for 5% of initial error.
//   - For medium‑scale metrics (Normalized Euclidean, Cosine, MSE) we aim for 1%.
//   - For divergence metrics (KL, JS, Chi‑squared) we aim for 10%.
//   - Fallback to the user‑supplied values from the config.
//
// Returns:
//   - fitnessThresh:  if best fitness drops below this, we consider the solution good enough.
//   - improveThresh:  if the relative improvement over a sliding window falls below this,
//     we assume stagnation and trigger a reheating.
func (ctx *AnnealingContext) GetEffectiveThresholds() (fitnessThresh, improveThresh float64) {
	baseScale := ctx.InitialFitness
	if baseScale < EPSILON {
		baseScale = EPSILON // prevent division by zero in scaling
	}

	switch ctx.Config.Distance {
	case "EUCLIDEAN", "MANHATTEN":
		// These metrics produce values that can be large; aim for tight convergence.
		fitnessThresh = baseScale * 0.05  // 5% of initial error
		improveThresh = baseScale * 0.005 // 0.5% improvement
	case "NORM_EUCLIDEAN", "COSINE", "MSE":
		// These are typically in a moderate range.
		fitnessThresh = baseScale * 0.01
		improveThresh = baseScale * 0.001
	case "KLDivergence", "JSDIVERGENCE", "CHI_SQUARED":
		// Divergence measures can be more sensitive; allow more slack.
		fitnessThresh = baseScale * 0.1
		improveThresh = baseScale * 0.01
	default:
		// If the metric is not recognised, fall back to the config values.
		fitnessThresh = ctx.Config.FitnessThreshold
		improveThresh = ctx.Config.MinImprovement
	}
	return
}

// ============================================================================
// DISTANCE FUNCTIONS – TYPE AND VALIDATION
// ============================================================================

// DistanceFunc is the signature for all distance/error functions.
// It takes three slices: constraints (target), testData (current synthetic totals),
// and weights (per‑variable importance). It returns a single float64: lower is better.
type DistanceFunc func([]float64, []float64, []float64) float64

// validatePopulationInputs performs a one‑time validation of all inputs
// to a syntheticPopulation call. It checks:
//   - constraint values are not empty
//   - weights (if provided) have the same length as constraints
//   - every microdata record has the same length as the constraints
//
// This validation is done once per area, not inside the inner distance loops,
// which greatly improves performance.
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
	return nil
}

// distanceFunc is a factory that returns the appropriate DistanceFunc
// based on the metric name given in the configuration.
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
	case "KLDivergence":
		return KLDivergence
	default:
		// If an unknown metric is requested, fall back to Euclidean.
		return EuclideanDistance
	}
}

// ============================================================================
// DISTANCE METRIC IMPLEMENTATIONS
// ============================================================================
//
// All distance functions assume that the inputs have already been validated
// (lengths match, weights are sane). They do not perform validation on every
// call for performance reasons. Each function supports optional weights:
// if the weights slice is shorter than the data, the missing weights are treated as 1.0.

// CanberraDistance computes the Canberra distance, which is a weighted
// sum of |x_i - y_i| / (|x_i| + |y_i|). It is robust to outliers.
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

// MeanSquaredError computes the weighted mean squared error between
// constraints and testData.
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

// Cosine computes the cosine distance: 1 - (p·q)/(||p||·||q||).
// It returns a value in [0,2] where 0 means identical direction.
// Added division‑by‑zero protection: if either norm is zero, the distance is 1.0.
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
	// FIX: Prevent division by zero.
	if denominator < EPSILON {
		return 1.0
	}
	return 1 - (dot / denominator)
}

// JSdivergence computes the Jensen‑Shannon divergence, a symmetric version
// of KL divergence. It calculates the midpoint distribution m = (p+q)/2
// and returns 0.5 * (KL(p||m) + KL(q||m)).
// Optimised to avoid allocating a new slice for m; it computes contributions
// on the fly.
func JSdivergence(constraints, testData, weights []float64) float64 {
	var sum float64
	for i := range constraints {
		mid := (constraints[i] + testData[i]) / 2

		// KL(constraints || mid)
		p := constraints[i] + EPSILON
		q := mid + EPSILON
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		sum += weight * p * math.Log(p/q)

		// KL(testData || mid)
		p = testData[i] + EPSILON
		q = mid + EPSILON
		sum += weight * p * math.Log(p/q)
	}
	return 0.5 * sum
}

// KLDivergence computes the Kullback‑Leibler divergence D(p||q) = Σ p_i log(p_i/q_i).
// To avoid log(0), both p and q are added a small EPSILON.
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

// ChiSquaredDistance computes Σ w_i * (observed - expected)² / expected.
// It uses EPSILON to avoid division by zero when expected is zero.
func ChiSquaredDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		observed := testData[i] + EPSILON
		expected := constraints[i] + EPSILON
		diff := observed - expected
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		distance += weight * (diff * diff) / expected
	}
	return distance
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
// This is a compromise between Manhattan (p=1) and Euclidean (p=2).
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

// NormalizedEuclideanDistance normalises each component by the constraint value
// (or a large penalty if the constraint is near zero). This makes the distance
// scale‑invariant.
func NormalizedEuclideanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		norm := constraints[i]
		weight := 1.0
		if len(weights) > i {
			weight = weights[i]
		}
		if math.Abs(norm) < EPSILON {
			// If the constraint is zero, any deviation is heavily penalised.
			if math.Abs(testData[i]) > EPSILON {
				distance += weight * 1000.0 * testData[i] * testData[i]
			}
			continue
		}
		diff := (testData[i] - constraints[i]) / norm
		distance += weight * diff * diff
	}
	return math.Sqrt(distance)
}

// WeightedPenaltyDistance applies a custom penalty based on the constraint value:
// very small constraints (<0.01) get a penalty factor 1000, small ones (<0.1) get 100.
// This is then multiplied by the weight.
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

// CustomWeightedDistance combines the penalty logic from WeightedPenaltyDistance
// with the weight vector, but here the penalty multiplies the weight itself.
func CustomWeightedDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		error := math.Abs(testData[i] - constraints[i])
		baseWeight := 1.0
		if len(weights) > i {
			baseWeight = weights[i]
		}
		if constraints[i] < 0.01 {
			baseWeight *= 1000.0
		} else if constraints[i] < 0.1 {
			baseWeight *= 100.0
		}
		distance += baseWeight * error
	}
	return distance
}

// AMeanSquaredError is a simple unweighted mean squared error, used internally
// for the final fitness value in the result struct.
func AMeanSquaredError(constraints, testData []float64) float64 {
	if len(constraints) != len(testData) {
		panic("slices must have the same length")
	}
	sumSquares := 0.0
	for i := range constraints {
		difference := constraints[i] - testData[i]
		sumSquares += difference * difference
	}
	return sumSquares / float64(len(constraints))
}

// ============================================================================
// MICRODATA VALIDATION
// ============================================================================

// isValidMicrodata checks whether a given microdata record can be used for
// an area with the given constraints. The only rule currently enforced is:
// if a constraint value is zero, the microdata value must also be zero.
// This prevents, for example, assigning a person with a positive value to
// a category that should be empty.
func isValidMicrodata(mdValues, constraints []float64) bool {
	for i, constraintVal := range constraints {
		if constraintVal == 0 && mdValues[i] != 0 {
			return false
		}
	}
	return true
}

// getValidIndices scans the microdata slice once and returns the indices of
// all records that satisfy isValidMicrodata. This cache is reused throughout
// the annealing process to avoid repeated scans.
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
// SIMULATED ANNEALING CORE FUNCTIONS
// ============================================================================

// replace performs a single replacement step in the synthetic population.
// It selects a random position in the population and swaps the microdata record
// there with another record that is valid for the area constraints.
//
// Parameters:
//   - microdata: the full set of available microdata records
//   - validIndices: pre‑computed list of indices that are valid for this area
//   - constraint: the target constraint values
//   - synthPopTotals: current aggregate totals of the synthetic population (updated in place)
//   - synthPopMicrodataIndexes: current list of microdata indices in the population
//   - fitness: current fitness value (before the replacement)
//   - temp: current temperature for the Metropolis criterion
//   - rng: random number generator for this area (ensures reproducibility)
//   - distfunc: the chosen distance function
//   - weights: variable weights
//
// Returns:
//   - newFitness: the fitness after the attempted replacement
//   - accepted: true if the replacement was accepted, false if reverted
//
// The function updates synthPopTotals and synthPopMicrodataIndexes in place.
// The Metropolis criterion accepts improvements (delta < 0) and also accepts
// equal fitness (delta == 0) to allow exploration of flat landscapes.
func replace(microdata []MicroData, validIndices []int, constraint ConstraintData, synthPopTotals []float64,
	synthPopMicrodataIndexes []int, fitness float64, temp float64, rng *rand.Rand, distfunc DistanceFunc, weights []float64) (float64, bool) {

	// If there are no valid records, we cannot perform a replacement.
	if len(validIndices) == 0 {
		return fitness, false
	}

	// Choose a new record uniformly from the valid indices (O(1)).
	randomIndex := rng.Intn(len(validIndices))
	randomReplacementIndex := validIndices[randomIndex]
	newValues := microdata[randomReplacementIndex].Values

	// Choose a random position in the current population to replace.
	randomReplaceIndex := rng.Intn(len(synthPopMicrodataIndexes))
	oldIndex := synthPopMicrodataIndexes[randomReplaceIndex]
	oldValues := microdata[oldIndex].Values

	// Update the aggregate totals: remove old record, add new record.
	for i := range synthPopTotals {
		synthPopTotals[i] = synthPopTotals[i] - oldValues[i] + newValues[i]
	}

	// Compute new fitness.
	newFitness := distfunc(constraint.Values, synthPopTotals, weights)
	delta := newFitness - fitness

	// Metropolis acceptance criterion.
	// Accept if the solution is better or equal (delta <= 0),
	// or if a random number is less than exp(-delta/temp) for worse solutions.
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

// initPopulation creates an initial synthetic population for a given area.
// It randomly selects `populationSize` microdata records (with replacement)
// from the set of valid records for that area.
//
// Parameters:
//   - constraint: area constraints (contains Total population size)
//   - microdata: all microdata records
//   - rng: random number generator (must be passed for reproducibility)
//
// Returns:
//   - synthPopTotals: initial aggregate totals
//   - synthPopMicrodataIndexes: initial list of selected indices
//   - ok: true if successful, false if no valid records or population size <= 0
//
// The population size is rounded from constraint.Total to handle floating‑point
// values safely.
func initPopulation(constraint ConstraintData, microdata []MicroData, rng *rand.Rand) ([]float64, []int, bool) {
	synthPopTotals := make([]float64, len(constraint.Values))

	// Round the total to an integer and protect against non‑positive values.
	populationSize := int(math.Round(constraint.Total))
	if populationSize <= 0 {
		return synthPopTotals, []int{}, false
	}

	synthPopMicrodataIndexes := make([]int, 0, populationSize)
	validIndices := getValidIndices(microdata, constraint)

	if len(validIndices) == 0 {
		return synthPopTotals, synthPopMicrodataIndexes, false
	}

	// Fill the population by random sampling from valid indices.
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

// normalizeFitness scales a raw fitness value relative to the initial fitness.
// This can be useful for reporting or for internal thresholds, though it's not
// currently used in the main loop.
func normalizeFitness(rawFitness float64, initialFitness float64) float64 {
	if initialFitness < EPSILON {
		return rawFitness
	}
	normalized := rawFitness / initialFitness
	if normalized > 1.0 {
		return 1.0
	}
	return normalized
}

// validateConfig checks that the annealing configuration parameters are valid.
// It returns an error if any parameter is out of the allowed range.
// Special handling: ReheatFactor is a percentage increase, so it can be zero
// or positive; a value of 0.3 means a 30% increase.
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
	if config.Change < 0 {
		return fmt.Errorf("Change must be non-negative, got %d", config.Change)
	}
	return nil
}

// ============================================================================
// MAIN SYNTHETIC POPULATION FUNCTION
// ============================================================================

// syntheticPopulation is the core of the simulated annealing algorithm.
// It takes a single constraint area and generates a synthetic population
// by repeatedly applying the `replace` operation while gradually cooling
// the temperature.
//
// Parameters:
//   - constraint: the area constraints
//   - microdata: all available microdata records
//   - config: annealing parameters
//   - rng: random number generator (must be deterministic for reproducibility)
//   - weights: variable weights for distance calculation
//
// Returns:
//   - results: the final synthetic population aggregates, IDs, and fitness
//   - bool: true if successful, false if no valid records exist
//
// The algorithm proceeds as follows:
//  1. Validate inputs and configuration.
//  2. Build an initial population.
//  3. Cache valid microdata indices.
//  4. Main loop:
//     - Perform a replacement.
//     - Track the best solution.
//     - Update the improvement window.
//     - If stagnation is detected, reheat.
//     - Cool the temperature.
//  5. Return the best solution found.
//
// Several key fixes are embedded:
//   - The RNG is passed in (no global rand).
//   - The window is not pre‑filled, preventing incorrect stagnation detection.
//   - The ReheatFactor is used as a percentage (1+ReheatFactor multiplier).
//   - A labeled break (MainLoop) ensures clean exit on stagnation.
//   - Variable scoping uses explicit `accepted` to avoid shadowing.
func syntheticPopulation(constraint ConstraintData, microdata []MicroData, config AnnealingConfig, rng *rand.Rand, weights []float64) (results, bool) {
	var synthPopResults results

	// Validate configuration and inputs once.
	if err := validateConfig(config); err != nil {
		panic(fmt.Sprintf("Invalid configuration: %v", err))
	}
	if err := validatePopulationInputs(constraint, microdata, weights); err != nil {
		panic(fmt.Sprintf("Invalid inputs: %v", err))
	}

	// Create initial population using the provided RNG.
	synthPopTotals, synthPopIDs, ok := initPopulation(constraint, microdata, rng)
	if !ok {
		return synthPopResults, false
	}

	// Cache valid indices for this area.
	validIndices := getValidIndices(microdata, constraint)
	if len(validIndices) == 0 {
		return synthPopResults, false
	}

	// Initial fitness.
	distfunc := distanceFunc(config)
	fitness := distfunc(constraint.Values, synthPopTotals, weights)

	// Setup annealing context for adaptive thresholds.
	annealingCtx := &AnnealingContext{
		InitialFitness: fitness,
		BestFitness:    fitness,
		Config:         config,
	}

	fitnessThreshold, improvementThreshold := annealingCtx.GetEffectiveThresholds()

	changes := config.Change
	temp := config.InitialTemp

	// Rolling window for stagnation detection – starts empty.
	improvementWindow := make([]float64, config.WindowSize)
	windowIndex := 0
	windowFilled := false
	iterationCount := 0

	// Best solution tracking.
	bestFitness := fitness
	bestSynthPopTotals := make([]float64, len(synthPopTotals))
	copy(bestSynthPopTotals, synthPopTotals)
	bestSynthPopIDs := make([]int, len(synthPopIDs))
	copy(bestSynthPopIDs, synthPopIDs)

	// Main loop – label allows breaking out completely.
MainLoop:
	for iteration := 0; iteration < config.MaxIterations && changes > 0 && temp > config.MinTemp; iteration++ {
		var accepted bool
		// Perform one replacement step.
		fitness, accepted = replace(microdata, validIndices, constraint, synthPopTotals, synthPopIDs, fitness, temp, rng, distfunc, weights)
		iterationCount++

		// Update best solution if improved.
		if fitness < bestFitness {
			bestFitness = fitness
			annealingCtx.BestFitness = bestFitness
			copy(bestSynthPopTotals, synthPopTotals)
			copy(bestSynthPopIDs, synthPopIDs)

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

			// Relative improvement over the window.
			relativeImprovement := (windowWorst - windowBest) / windowWorst

			// If progress is too slow, reheat.
			if relativeImprovement < improvementThreshold {
				// ReheatFactor is a percentage: e.g., 0.3 => multiply by 1.3.
				temp = math.Max(temp*(1+config.ReheatFactor), config.InitialTemp*0.1)

				// If stagnation is extreme, stop entirely.
				if relativeImprovement < improvementThreshold/100000 {
					break MainLoop
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

	// Prepare the result struct with the best solution found.
	synthPopResults.area = constraint.ID
	synthPopResults.synthpop_totals = bestSynthPopTotals
	synthPopResults.ids = make([]string, len(bestSynthPopIDs))
	for i, id := range bestSynthPopIDs {
		synthPopResults.ids[i] = microdata[id].ID
	}

	synthPopResults.constraint_totals = constraint.Values
	// Use MSE for the final fitness value.
	synthPopResults.fitness = MeanSquaredError(constraint.Values, bestSynthPopTotals, weights)
	synthPopResults.population = constraint.Total

	return synthPopResults, true
}
