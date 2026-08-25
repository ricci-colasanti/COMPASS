package main

import (
	"math"
	"math/rand"
)

// Constants defining distance metrics and numerical stability parameters
const (
	// EPSILON is a small value to prevent division by zero and ensure numerical stability
	EPSILON = 1e-10

	// Distance metric types
	KL_DIVERGENCE  = iota // Kullback-Leibler divergence
	CHI_SQUARED           // Chi-squared distance
	EUCLIDEAN             // Standard Euclidean distance
	NORM_EUCLIDEAN        // Normalized Euclidean distance
	MANHATTEN             // Manhattan distance
)

// CHANGED: Added AnnealingContext for adaptive thresholds
type AnnealingContext struct {
	InitialFitness float64
	BestFitness    float64
	Config         AnnealingConfig
}

// CHANGED: Added method to get metric-aware thresholds
func (ctx *AnnealingContext) GetEffectiveThresholds() (fitnessThresh, improveThresh float64) {
	// Use relative thresholds based on initial fitness and metric type
	baseScale := ctx.InitialFitness
	if baseScale < EPSILON {
		baseScale = EPSILON
	}

	switch ctx.Config.Distance {
	case "EUCLIDEAN", "MANHATTEN":
		// Larger scale metrics - use smaller relative thresholds
		fitnessThresh = baseScale * 0.05  // Aim for 5% of initial error
		improveThresh = baseScale * 0.005 // 0.5% improvement threshold
	case "NORM_EUCLIDEAN", "COSINE", "MSE":
		// Medium scale metrics
		fitnessThresh = baseScale * 0.01  // Aim for 1% of initial error
		improveThresh = baseScale * 0.001 // 0.1% improvement threshold
	case "KLDivergence", "JSDIVERGENCE", "CHI_SQUARED":
		// Divergence metrics - use larger relative thresholds
		fitnessThresh = baseScale * 0.1  // Aim for 10% of initial error
		improveThresh = baseScale * 0.01 // 1% improvement threshold
	default:
		// Fallback to config values if no specific scaling
		fitnessThresh = ctx.Config.FitnessThreshold
		improveThresh = ctx.Config.MinImprovement
	}
	return
}

type DistanceFunc func([]float64, []float64, []float64) float64

func distanceFunc(config AnnealingConfig) DistanceFunc {
	// distanceFunc returns the appropriate distance calculation function based on the configured metric.
	// It serves as a factory function for distance metrics used in simulated annealing.
	//
	// Parameters:
	//   - config: AnnealingConfig containing the distance metric specification
	//
	// Returns:
	//   - DistanceFunc: The selected distance calculation function
	//
	// Supported metrics:
	//   - "CHI_SQUARED": Chi-squared distance
	//   - "EUCLIDEAN": Standard Euclidean distance
	//   - "NORM_EUCLIDEAN": Normalized Euclidean distance
	//   - "MANHATTAN": Manhattan distance (L1 norm)
	//   - Default: KL Divergence
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
		return EuclideanDistance
	}
}

func CanberraDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		numerator := math.Abs(testData[i] - constraints[i])
		denominator := math.Abs(testData[i]) + math.Abs(constraints[i])
		if denominator > 0 { // Avoid division by zero
			distance += weights[i] * numerator / denominator
		}
	}
	return distance
}

func MeanSquaredError(constraints, testData, weights []float64) float64 {
	if len(constraints) != len(testData) || len(constraints) != len(weights) {
		panic("slices must have the same length")
	}

	sumSquares := 0.0
	totalWeight := 0.0
	for i := range constraints {
		difference := constraints[i] - testData[i]
		sumSquares += weights[i] * difference * difference
		totalWeight += weights[i]
	}

	if totalWeight == 0 {
		return 0
	}
	return sumSquares / totalWeight
}

func Cosine(constraints, testData, weights []float64) float64 {
	dot, normConstraints, normTestData := 0.0, 0.0, 0.0
	for i := range constraints {
		dot += weights[i] * constraints[i] * testData[i]
		normConstraints += weights[i] * constraints[i] * constraints[i]
		normTestData += weights[i] * testData[i] * testData[i]
	}
	return 1 - (dot / (math.Sqrt(normConstraints) * math.Sqrt(normTestData)))
}

func JSdivergence(constraints, testData, weights []float64) float64 {
	// Compute the midpoint distribution
	m := make([]float64, len(constraints))
	for i := range constraints {
		m[i] = (constraints[i] + testData[i]) / 2
	}
	// Symmetrized KL divergence
	return 0.5 * (KLDivergence(constraints, m, weights) + KLDivergence(testData, m, weights))
}

// KLDivergence calculates the Kullback-Leibler divergence between two distributions
func KLDivergence(constraints, testData, weights []float64) float64 {
	divergence := 0.0
	for i := range constraints {
		p := constraints[i] + EPSILON
		q := testData[i] + EPSILON
		divergence += weights[i] * p * math.Log(p/q)
	}
	return divergence
}

// ChiSquaredDistance calculates the chi-squared distance between observed and expected values
func ChiSquaredDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		observed := testData[i] + EPSILON
		expected := constraints[i] + EPSILON
		diff := observed - expected
		distance += weights[i] * (diff * diff) / expected
	}
	return distance
}

// EuclideanDistance calculates the standard Euclidean distance between two vectors
func EuclideanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		diff := testData[i] - constraints[i]
		distance += weights[i] * diff * diff
	}
	return math.Sqrt(distance)
}

// MinkowskiDistance - Euclidean when p=2, Manhattan when p=1
func MinkowskiDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	p := 1.5
	for i := range constraints {
		diff := math.Abs(testData[i] - constraints[i])
		distance += weights[i] * math.Pow(diff, p)
	}
	return math.Pow(distance, 1/p)
}

// NormalizedEuclideanDistance calculates a normalized version of Euclidean distance
func NormalizedEuclideanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		norm := constraints[i]
		if math.Abs(norm) < EPSILON {
			if math.Abs(testData[i]) > EPSILON {
				distance += weights[i] * 1000.0 * testData[i] * testData[i]
			}
			continue
		}
		diff := (testData[i] - constraints[i]) / norm
		distance += weights[i] * diff * diff
	}
	return math.Sqrt(distance)
}

// WeightedPenaltyDistance with explicit weights
func WeightedPenaltyDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		error := math.Abs(testData[i] - constraints[i])

		// Custom penalty rules combined with weights
		penalty := 1.0
		if constraints[i] < 0.01 { // Very small constraint
			penalty = 1000.0
		} else if constraints[i] < 0.1 { // Small constraint
			penalty = 100.0
		}
		distance += weights[i] * penalty * error
	}
	return distance
}

// ManhattanDistance calculates the Manhattan distance (L1 norm) between two vectors
func ManhattanDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		distance += weights[i] * math.Abs(testData[i]-constraints[i])
	}
	return distance
}

// New: Weighted version that combines your custom penalty with weights
func CustomWeightedDistance(constraints, testData, weights []float64) float64 {
	distance := 0.0
	for i := range constraints {
		error := math.Abs(testData[i] - constraints[i])

		// Use both the weight and your custom penalty logic
		baseWeight := weights[i]

		// Apply additional penalty based on constraint value
		if constraints[i] < 0.01 {
			baseWeight *= 1000.0
		} else if constraints[i] < 0.1 {
			baseWeight *= 100.0
		}

		distance += baseWeight * error
	}
	return distance
}

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

// isValidMicrodata checks if microdata values satisfy all constraints
//
// Parameters:
//   - mdValues: The microdata values to check
//   - constraints: The constraints to validate against
//
// Returns:
//   - true if all zero constraints are satisfied, false otherwise
func isValidMicrodata(mdValues, constraints []float64) bool {
	for i, constraintVal := range constraints {
		if constraintVal == 0 && mdValues[i] != 0 {
			return false
		}
	}
	return true
}

// replace performs a replacement operation in the synthetic population using simulated annealing
//
// Parameters:
//   - microdata: The source microdata records
//   - constraint: The area constraints
//   - synthPopTotals: Current aggregate statistics
//   - synthPopMicrodataIndexess: Current population indices
//   - fitness: Current fitness score
//   - temp: Current temperature
//   - rng: Random number generator
//
// Returns:
//   - newFitness: The fitness after replacement
//   - flag: True if replacement was accepted, false if reverted
func replace(microdata []MicroData, constraint ConstraintData, synthPopTotals []float64,
	synthPopMicrodataIndexess []int, fitness float64, temp float64, rng *rand.Rand, distfunc DistanceFunc, weights []float64) (float64, bool) {

	flag := true

	var randomReplacmentIndex int
	var newValues []float64
	validFound := false
	maxAttempts := 100

	// Find valid replacement candidate
	for attempts := 0; attempts < maxAttempts; attempts++ {
		randomReplacmentIndex = rng.Intn(len(microdata))
		newValues = microdata[randomReplacmentIndex].Values
		if isValidMicrodata(newValues, constraint.Values) {
			validFound = true
			break
		}
	}

	if !validFound {
		return fitness, false
	}

	// Perform replacement
	randomReplceIndex := rng.Intn(len(synthPopMicrodataIndexess))
	replacementIndex := synthPopMicrodataIndexess[randomReplceIndex]
	oldValues := microdata[replacementIndex].Values

	// Update aggregates
	for i := 0; i < len(synthPopTotals); i++ {
		synthPopTotals[i] = synthPopTotals[i] - oldValues[i] + newValues[i]
	}

	newFitness := distfunc(constraint.Values, synthPopTotals, weights)

	// Metropolis acceptance criterion
	if newFitness >= fitness || math.Exp((fitness-newFitness)/temp) < rng.Float64() {
		// Revert changes
		for i := 0; i < len(synthPopTotals); i++ {
			synthPopTotals[i] = synthPopTotals[i] - newValues[i] + oldValues[i]
		}
		newFitness = fitness
		flag = false
	} else {
		// Accept changes
		synthPopMicrodataIndexess[randomReplceIndex] = randomReplacmentIndex
	}

	return newFitness, flag
}

// initPopulation creates an initial synthetic population for an area
//
// Parameters:
//   - constraint: The area constraints
//   - microdata: The source microdata
//
// Returns:
//   - synthPopTotals: Initial aggregate statistics
//   - synthPopMicrodataIndexs: Indices of selected microdata records
func initPopulation(constraint ConstraintData, microdata []MicroData) ([]float64, []int, bool) {
	synthPopTotals := make([]float64, len(constraint.Values))
	synthPopMicrodataIndexs := make([]int, 0, int(constraint.Total))

	//Pre-filter valid microdata
	var validIndices []int
	for i, md := range microdata {
		if isValidMicrodata(md.Values, constraint.Values) {
			validIndices = append(validIndices, i)
		}
	}

	if len(validIndices) == 0 {
		//println("No valid microdata records match constraints")
		return synthPopTotals, synthPopMicrodataIndexs, false
	}
	//println("ok")

	// Create initial population
	for i := 0; i < int(constraint.Total); i++ {
		randomIndex := validIndices[rand.Intn(len(validIndices))]
		randomElement := microdata[randomIndex]

		synthPopMicrodataIndexs = append(synthPopMicrodataIndexs, randomIndex)
		for j := 0; j < len(synthPopTotals); j++ {
			synthPopTotals[j] += randomElement.Values[j]
		}
	}

	return synthPopTotals, synthPopMicrodataIndexs, true
}

func normalizeFitness(rawFitness float64, distanceMetric string, initialFitness float64) float64 {
	if initialFitness < EPSILON {
		return rawFitness
	}

	// Normalize to percentage of initial fitness (0-1 scale where 0 is perfect)
	normalized := rawFitness / initialFitness

	// For metrics where lower is better, this gives us 0=perfect, 1=initial, >1=worse
	// We can cap it at 1.0 to keep consistent scaling
	if normalized > 1.0 {
		return 1.0
	}
	return normalized
}

// CHANGED: syntheticPopulation now uses adaptive thresholds
func syntheticPopulation(constraint ConstraintData, microdata []MicroData, config AnnealingConfig, rng *rand.Rand, weights []float64) (results, bool) {
	var synthPopResults results

	// Initialize population and fitness
	synthPopTotals, synthPopIDs, flag := initPopulation(constraint, microdata)
	if flag {
		distfunc := distanceFunc(config)
		fitness := distfunc(constraint.Values, synthPopTotals, weights)

		// CHANGED: Create annealing context for adaptive thresholds
		annealingCtx := &AnnealingContext{
			InitialFitness: fitness,
			BestFitness:    fitness,
			Config:         config,
		}

		// CHANGED: Get metric-aware thresholds
		fitnessThreshold, improvementThreshold := annealingCtx.GetEffectiveThresholds()

		// Setup annealing parameters
		changes := config.Change
		temp := config.InitialTemp
		improvementWindow := make([]float64, config.WindowSize)
		windowIndex := 0
		bestFitness := fitness
		improvementWindow[windowIndex] = fitness
		windowIndex++

		// Track best solution
		bestSynthPopTotals := make([]float64, len(synthPopTotals))
		copy(bestSynthPopTotals, synthPopTotals)
		bestSynthPopIDs := make([]int, len(synthPopIDs))
		copy(bestSynthPopIDs, synthPopIDs)
		totaliterations := 0
		//flag := true

		// Main optimization loop
		for iteration := 0; iteration < config.MaxIterations && changes > 0 && temp > config.MinTemp; iteration++ {
			flag := true
			fitness, flag = replace(microdata, constraint, synthPopTotals, synthPopIDs, fitness, temp, rng, distfunc, weights)

			// Update best solution
			if fitness < bestFitness {
				bestFitness = fitness
				// CHANGED: Update context with new best fitness
				annealingCtx.BestFitness = bestFitness
				copy(bestSynthPopTotals, synthPopTotals)
				copy(bestSynthPopIDs, synthPopIDs)

				// CHANGED: Use adaptive fitness threshold instead of fixed one
				if bestFitness <= fitnessThreshold {
					// fmt.Printf("Area %s: Converged with fitness %.6f <= threshold %.6f\n",
					// constraint.ID, bestFitness, fitnessThreshold)
					break
				}
			}

			// Track improvements
			improvementWindow[windowIndex] = fitness
			windowIndex = (windowIndex + 1) % config.WindowSize

			// Check for stagnation
			if iteration >= config.WindowSize {
				windowBest, windowWorst := improvementWindow[0], improvementWindow[0]
				for _, val := range improvementWindow {
					if val < windowBest {
						windowBest = val
					}
					if val > windowWorst {
						windowWorst = val
					}
				}

				// CHANGED: Use adaptive improvement threshold
				relativeImprovement := (windowWorst - windowBest) / windowWorst
				if relativeImprovement < improvementThreshold {
					temp = math.Max(temp*(1+config.ReheatFactor), config.InitialTemp*0.1)

					// CHANGED: Also use adaptive threshold for complete stagnation
					if relativeImprovement < improvementThreshold/100000 {
						// fmt.Printf("Area %s: Stagnated with improvement %.6f < threshold %.6f\n",
						// 	constraint.ID, relativeImprovement, improvementThreshold)
						flag = false
						break
					}
				}
			}

			temp *= config.CoolingRate

			if !flag {
				changes--
			}
			totaliterations++
		}

		// Prepare results
		synthPopResults.area = constraint.ID
		synthPopResults.synthpop_totals = bestSynthPopTotals
		synthPopResults.ids = make([]string, len(bestSynthPopIDs))
		for i, id := range bestSynthPopIDs {
			synthPopResults.ids[i] = microdata[id].ID
		}

		synthPopResults.constraint_totals = constraint.Values
		synthPopResults.fitness = MeanSquaredError(constraint.Values, bestSynthPopTotals, weights)
		synthPopResults.population = constraint.Total

		// CHANGED: Better debugging output with metric context
		if synthPopResults.fitness > 10000 {
			// fmt.Printf("Area %s: High final fitness - Metric: %s, Iterations: %d, BestFitness: %.6f\n",
			// 	constraint.ID, config.Distance, totaliterations, bestFitness)
		}
	}
	return synthPopResults, flag
}
