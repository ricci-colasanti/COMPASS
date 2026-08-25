# Synthetic Population Generation Using Simulated Annealing

## Overview

This package implements a **Simulated Annealing (SA)** algorithm for generating synthetic populations from microdata records while satisfying area-level constraints. It's designed for spatial microsimulation and population synthesis tasks commonly used in agent-based modeling, transportation planning, and demographic studies.

## Table of Contents
1. [Core Concepts](#core-concepts)
2. [Architecture](#architecture)
3. [Simulated Annealing Algorithm](#simulated-annealing-algorithm)
4. [Distance Metrics](#distance-metrics)
5. [Key Components](#key-components)
6. [Configuration Parameters](#configuration-parameters)
7. [Performance Optimizations](#performance-optimizations)
8. [Usage Example](#usage-example)

---

## Core Concepts

### Problem Statement
Given:
- **Constraint data**: Target aggregate totals for each area (e.g., population counts by age/sex groups)
- **Microdata**: Individual-level records with attributes matching the constraints
- **Weights**: Optional importance weights for each attribute

The goal is to select a subset of microdata records for each area such that:
1. The aggregate totals of selected records match the constraint values as closely as possible
2. Each selected record satisfies the zero-constraint rules (if a constraint is 0, selected records must have 0 in that dimension)

### Simulated Annealing Overview
Simulated Annealing is a probabilistic optimization algorithm inspired by the annealing process in metallurgy. It explores the solution space by:
1. Starting with a random initial solution
2. Making small changes (replace one record with another)
3. Accepting worse solutions with a probability that decreases over time
4. Gradually "cooling" the temperature to converge to a good solution

---

## Architecture

### High-Level Flow

```
Input: Constraint Data + Microdata + Configuration
    ↓
1. Validate Inputs
    ↓
2. Initialize Population (Random selection)
    ↓
3. Cache Valid Microdata Indices
    ↓
4. Main SA Loop
    ├── Select random record to replace
    ├── Select valid replacement record
    ├── Update aggregate totals
    ├── Calculate new fitness
    ├── Metropolis Acceptance Criterion
    └── Update temperature
    ↓
5. Return Best Solution
```

### Directory Structure
```
simulatedAnnealing.go     # Core SA implementation
├── Constants
├── Types & Interfaces
├── Distance Metrics
├── Microdata Validation
├── SA Core Functions
└── Main Orchestration
```

---

## Simulated Annealing Algorithm

### 1. Initialization

The algorithm starts by creating an initial population:

```go
func initPopulation(constraint, microdata, rng)
```

**Process:**
- Calculate population size = `round(constraint.Total)`
- Find all microdata records that satisfy zero constraints
- Randomly select `populationSize` records from valid microdata
- Compute initial aggregate totals

**Key Feature**: Records are selected with replacement (same record can appear multiple times), which is valid for population synthesis.

### 2. Neighborhood Operation: Replace

The primary operation to explore the solution space:

```go
func replace(microdata, validIndices, constraint, synthPopTotals, ...)
```

**Process:**
1. Select a random position in the current population
2. Select a random valid microdata record (from pre-filtered indices)
3. Remove the old record's values from aggregates
4. Add the new record's values to aggregates
5. Compute new fitness score
6. Decide to accept or reject using Metropolis criterion

**Why This Works**: Each replacement changes exactly one record, creating a small perturbation that allows the algorithm to gradually improve the solution.

### 3. Metropolis Acceptance Criterion

The heart of simulated annealing:

```go
if delta <= 0 || math.Exp(-delta/temp) > rng.Float64() {
    // Accept the change
} else {
    // Reject and revert
}
```

**Logic:**
- **If improvement (delta < 0)**: Always accept
- **If equal fitness (delta == 0)**: Accept (allows exploration of flat landscapes)
- **If worse (delta > 0)**: Accept with probability `e^(-delta/temp)`

**Key Insight**: The probability of accepting worse solutions decreases as temperature drops, allowing the algorithm to escape local optima early and converge later.

### 4. Temperature Schedule

```go
// Cool down at each iteration
temp *= config.CoolingRate  // e.g., 0.999

// Reheat when stagnation detected
if relativeImprovement < improvementThreshold {
    temp = max(temp * (1 + ReheatFactor), InitialTemp * 0.1)
}
```

**Cooling**: Exponential decay of temperature
**Reheating**: When progress stalls, increase temperature to escape local optima

### 5. Stagnation Detection

The algorithm monitors improvement over a sliding window:

```go
windowSize = config.WindowSize
relativeImprovement = (windowWorst - windowBest) / windowWorst

if relativeImprovement < improvementThreshold {
    // No significant improvement - reheat
}
```

**Purpose**: Detects when the algorithm is stuck in a local optimum and triggers reheating to escape.

### 6. Convergence Criteria

The algorithm stops when ANY of these conditions are met:

| Condition | Description |
|-----------|-------------|
| `bestFitness <= fitnessThreshold` | Target fitness achieved |
| `changes <= 0` | No successful replacements |
| `temp <= config.MinTemp` | Temperature too low |
| `iteration >= config.MaxIterations` | Maximum iterations reached |
| `relativeImprovement < improvementThreshold/100000` | Complete stagnation |

---

## Distance Metrics

### Overview

Distance metrics measure how well the synthetic population matches the constraints. Lower values indicate better matches.

### Supported Metrics

| Metric | Formula | Use Case |
|--------|---------|----------|
| **Euclidean** | `√(Σ wᵢ(xᵢ-yᵢ)²)` | Standard L2 norm |
| **Manhattan** | `Σ wᵢ|xᵢ-yᵢ|` | Robust to outliers |
| **MSE** | `(1/Σw)Σ wᵢ(xᵢ-yᵢ)²` | Average squared error |
| **Chi-Squared** | `Σ wᵢ((observed-expected)²/expected)` | Goodness of fit |
| **KL Divergence** | `Σ wᵢ p log(p/q)` | Distribution similarity |
| **JS Divergence** | `0.5*(KL(p,m)+KL(q,m))` | Symmetric KL divergence |
| **Cosine** | `1 - (p·q)/(||p||||q||)` | Angular similarity |
| **Normalized Euclidean** | See code | Scale-invariant L2 |

### Adaptive Thresholds

Different metrics have different scales, so thresholds are adjusted automatically:

```go
func GetEffectiveThresholds() {
    switch metric {
    case "EUCLIDEAN", "MANHATTEN":
        threshold = initialFitness * 0.05    // 5% of initial error
    case "NORM_EUCLIDEAN", "COSINE":
        threshold = initialFitness * 0.01    // 1% of initial error
    case "KLDivergence", "CHI_SQUARED":
        threshold = initialFitness * 0.1     // 10% of initial error
    }
}
```

---

## Key Components

### 1. AnnealingContext

```go
type AnnealingContext struct {
    InitialFitness float64      // Fitness at start
    BestFitness    float64      // Best fitness found
    Config         AnnealingConfig // Configuration
}
```

**Purpose**: Maintains state and provides metric-aware threshold calculations.

### 2. DistanceFunc

```go
type DistanceFunc func([]float64, []float64, []float64) float64
```

**Purpose**: Standard interface for all distance metrics. Parameters: constraints, testData, weights.

### 3. Microdata Validation

```go
func isValidMicrodata(mdValues, constraints []float64) bool
```

**Purpose**: Ensures that if a constraint is zero, the microdata record also has zero in that dimension. This is critical for population synthesis where certain attributes must be absent.

---

## Configuration Parameters

### Required Configuration

```json
{
    "MaxIterations": 100000,    // Maximum SA iterations
    "InitialTemp": 1000.0,      // Starting temperature
    "MinTemp": 0.01,            // Minimum temperature
    "CoolingRate": 0.999,       // Temperature decay rate (0-1)
    "ReheatFactor": 0.3,        // Temperature increase on stagnation (percentage)
    "WindowSize": 100,          // Stagnation detection window
    "Change": 1000,             // Maximum failed attempts before stopping
    "Distance": "EUCLIDEAN",    // Distance metric to use
    "FitnessThreshold": 1.0,    // Target fitness to stop
    "MinImprovement": 0.001     // Minimum improvement threshold
}
```

### Parameter Tuning Guide

| Parameter | Effect | Tuning Advice |
|-----------|--------|---------------|
| **InitialTemp** | Higher = more exploration | Start high (1000-10000) for complex problems |
| **CoolingRate** | Controls convergence speed | 0.999-0.9999 for slow cooling; 0.99-0.999 for fast |
| **ReheatFactor** | Escape from local optima | 0.1-0.5 (10-50% increase) |
| **WindowSize** | Stagnation sensitivity | 50-200 (larger = less sensitive) |
| **Change** | Stopping tolerance | 100-5000 (higher = more patience) |

---

## Performance Optimizations

### 1. Input Validation Once

**Before**: Each distance function called validated inputs → O(n²) overhead
**After**: Single validation at start → O(n) overhead

```go
// Called once at initialization
validatePopulationInputs(constraint, microdata, weights)
```

### 2. Cached Valid Indices

**Before**: 100-attempt rejection sampling for each replacement
**After**: Direct O(1) lookup from pre-filtered indices

```go
validIndices := getValidIndices(microdata, constraint)  // Once
randomIndex := validIndices[rng.Intn(len(validIndices))]  // O(1)
```

### 3. Optimized Distance Calculations

| Optimization | Benefit |
|--------------|---------|
| No per-iteration validation | 10-30% speedup |
| Inline weight handling | Reduced branching |
| JSdivergence without allocation | Less GC pressure |
| Division by zero protection | Stable execution |

### 4. Thread Safety

```go
// Pass RNG to each function instead of using global rand
func syntheticPopulation(..., rng *rand.Rand, ...)
```

**Why**: Enables safe parallel execution across multiple areas.

---

## Usage Example

### Basic Usage

```go
// 1. Load data
constraints := loadConstraints("constraints.csv")
microdata := loadMicrodata("microdata.csv")

// 2. Configure annealing
config := AnnealingConfig{
    MaxIterations: 50000,
    InitialTemp: 1000.0,
    MinTemp: 0.1,
    CoolingRate: 0.999,
    ReheatFactor: 0.3,
    WindowSize: 50,
    Change: 1000,
    Distance: "EUCLIDEAN",
    FitnessThreshold: 0.01,
    MinImprovement: 0.001,
}

// 3. Create RNG
rng := rand.New(rand.NewSource(time.Now().UnixNano()))

// 4. Run for each area
for _, constraint := range constraints {
    results, ok := syntheticPopulation(constraint, microdata, config, rng, weights)
    if ok {
        fmt.Printf("Area %s: Fitness = %.6f\n", results.area, results.fitness)
        // Use results.ids to get selected microdata records
    }
}
```

### Advanced: Parallel Processing

```go
// Process multiple areas concurrently
var wg sync.WaitGroup
results := make([]results, len(constraints))

for i, constraint := range constraints {
    wg.Add(1)
    go func(idx int, c ConstraintData) {
        defer wg.Done()
        // Each goroutine gets its own RNG
        rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(idx)))
        results[idx], _ = syntheticPopulation(c, microdata, config, rng, weights)
    }(i, constraint)
}
wg.Wait()
```

---

## Algorithm Walkthrough: Step-by-Step Example

### Scenario
- **Constraint**: Need 100 people with 50 males, 50 females
- **Microdata**: 1000 individual records with gender attribute
- **Goal**: Select 100 records with exactly 50 males and 50 females

### Step 1: Initialize
```
Population: 100 random records (e.g., 45 males, 55 females)
Fitness: 25 (since |45-50| + |55-50| = 10)
Temperature: 1000
```

### Step 2: Replace Operation
```
1. Select random position (position 42, currently female)
2. Select replacement record (male from microdata)
3. Update: 46 males, 54 females
4. New Fitness: 8 (improvement!)
5. Accept (delta < 0)
```

### Step 3: Worse Solution Acceptance
```
1. Select random position (position 15, currently male)
2. Select replacement record (female)
3. Update: 45 males, 55 females
4. New Fitness: 10 (worse!)
5. Probability = exp(-(10-8)/1000) = 0.998
6. Random 0.5 < 0.998 → Accept (escapes local optimum)
```

### Step 4: Cooling
```
Temperature decreases: 1000 → 999
As temp drops, accepting worse solutions becomes harder
Algorithm converges toward 50/50 split
```

### Step 5: Convergence
```
Best fitness found: 0 (perfect match!)
Algorithm terminates early due to fitness threshold
```

---

## Error Handling

### Input Validation
```go
if err := validateConfig(config); err != nil {
    panic(fmt.Sprintf("Invalid configuration: %v", err))
}
```

### Division by Zero Protection
```go
if denominator < EPSILON {
    return 1.0  // Safe default
}
```

### Empty Data Handling
```go
if len(validIndices) == 0 {
    return results, false  // No valid microdata found
}
```

---

## Debugging and Monitoring

### Key Metrics to Track

| Metric | How to Check |
|--------|--------------|
| Fitness improvement | Compare initial vs final fitness |
| Temperature trajectory | Log temp at each iteration |
| Acceptance rate | Track accepted/total replacements |
| Stagnation events | Count reheat triggers |
| Convergence time | Total iterations taken |

