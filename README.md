<div align="center">
  <img src="img/COMPASS.png" alt="COMPASS Logo" width="400"/>
</div>

# COMPASS - Spatial Synthetic Population Generator

**Work in Progress - Not Ready for Production Use**

*This is a research/experimental project for generating spatial synthetic populations. The code is under active development and not yet intended for public release or real-world use. But is getting there!*

---

## Overview

COMPASS is a Go-based tool for generating synthetic populations by combining census data with survey microdata using simulated annealing optimization. It creates spatially detailed synthetic populations that match statistical constraints from census data while preserving individual characteristics from survey microdata.

### How It Works

The tool uses **simulated annealing**, a probabilistic optimization algorithm, to select a subset of microdata records for each geographic area:

1. **Input Loading**: Reads constraint data (census totals), microdata (survey records), and geographical groupings
2. **Initialization**: Creates a random population by sampling microdata records for each area
3. **Optimization Loop**:
   - Proposes changes by swapping one microdata record with another
   - Accepts or rejects changes based on fitness improvement and current temperature
   - Gradually cools the temperature to converge on a good solution
   - Reheats if progress stalls to escape local optima
4. **Output Generation**: Writes final population mappings and validation data

### Key Features

- **Parallel Processing**: Utilizes all CPU cores for fast population generation
- **Multiple Distance Metrics**: KL Divergence, Chi-Squared, Euclidean, Cosine, MSE, and more
- **Deterministic Mode**: Reproducible results with fixed random seeds
- **Adaptive Thresholds**: Automatically adjusts convergence criteria based on the chosen metric
- **Multi-Language API**: Call from Python, R, or directly via command line
- **UK-Focused Design**: Optimized for UK census geography and Understanding Society data

---

## Installation

### Pre-compiled Binaries

Pre-compiled executables are available for:

| Platform | Status | Location |
|----------|--------|----------|
| Linux (64-bit) | ✅ Available | Included in repository |
| Windows | 🚧 Coming soon | Build from source |
| macOS | 🚧 Coming soon | Build from source |

### Prerequisites (Building from Source)

- **Go 1.18+** (required for building)
- **Python 3.6+** (optional, for Python interface)
- **R 4.0+ with jsonlite package** (optional, for R interface)

### Quick Start (Using Pre-compiled Binary)

```bash
# Download the Linux binary
chmod +x compass

# Test it works
./compass -f config.json

# Or pipe JSON directly
echo '{"constraints":"data.csv","microdata":"micro.csv"}' | ./compass
```

### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd compass

# Build the binary
go build -o compass .

# Make it executable (Linux/macOS)
chmod +x compass

# Test the build
./compass -f config.json
```

**Note for Windows users**: When building from source, the output will be `compass.exe`. Run with `compass.exe -f config.json`.

---

## Usage

### Command Line Interface

The tool accepts JSON configuration either from a file or via stdin:

```bash
# Method 1: JSON file input
./compass -f config.json

# Method 2: Pipe JSON to stdin
echo '{"constraints":"data.csv","microdata":"micro.csv"}' | ./compass

# Method 3: Run with GUI (placeholder)
./compass -g
```

### Python Interface

```python
#!/usr/bin/env python3
import json
import subprocess
import sys
from pathlib import Path

def run_compass(config_dict, binary_path="./compass"):
    """
    Run COMPASS from Python with a configuration dictionary.
    
    Args:
        config_dict: Dictionary containing COMPASS configuration parameters
        binary_path: Path to compiled COMPASS binary (default: "./compass")
    
    Returns:
        Dictionary with results including status, message, and log
    """
    # Convert to compact JSON
    json_input = json.dumps(config_dict, separators=(',', ':'), ensure_ascii=False)
    
    try:
        # Execute COMPASS
        result = subprocess.run(
            [binary_path],
            input=json_input,
            capture_output=True,
            text=True,
            check=False
        )
        
        # Parse JSON response
        if result.stdout:
            response = json.loads(result.stdout)
        else:
            response = {"status": "error", "message": "No output from COMPASS"}
        
        # Add execution details
        response["return_code"] = result.returncode
        if result.stderr:
            response["stderr"] = result.stderr.splitlines()
        
        return response
        
    except json.JSONDecodeError as e:
        return {
            "status": "error",
            "message": f"Failed to parse COMPASS output: {str(e)}",
            "raw_output": result.stdout if 'result' in locals() else None
        }
    except Exception as e:
        return {
            "status": "error", 
            "message": f"Failed to execute COMPASS: {str(e)}"
        }

# Example usage
if __name__ == "__main__":
    config = {
        "constraints" : "data/BlockLand/artifical_cencus.csv",
        "microdata"   : "data/BlockLand/artifical_hh_survay.csv",
        "groups"      : "data/BlockLand/artificial_groups.csv",
        "output"      : "results/artificial_synthetic_population.csv",
        "validate"    : "results/artificial_synthPopSurvey.csv",
        "initialTemp": 1000.0,
        "minTemp": 0.001,
        "coolingRate": 0.997,
        "reheatFactor": 0.3,
        "fitnessThreshold": 0.01,
        "minImprovement": 0.001,
        "maxIterations": 500000,
        "windowSize": 5000,
        "change": 100000,
        "distance": "NORM_EUCLIDEAN",
        "useRandomSeed": "no",
        "randomSeed": 42
    }
    
    result = run_compass(config)
    print("Status:", result.get("status"))
    print("Message:", result.get("message"))
    
    if "log" in result:
        print("\nExecution Log:")
        for line in result["log"]:
            print(f"  {line}")
```

### R Interface

```r
# R script: run_compass.R
library(jsonlite)

run_compass <- function(config_list, binary_path = "./compass") {
  #' Run COMPASS from R with a configuration list
  #'
  #' @param config_list List containing COMPASS configuration parameters
  #' @param binary_path Path to compiled COMPASS binary (default: "./compass")
  #' @return List with results including status, message, and execution details
  
  # Convert to compact JSON
  json_input <- toJSON(config_list, auto_unbox = TRUE, pretty = FALSE)
  
  # Execute COMPASS
  result <- system2(
    binary_path,
    input = json_input,
    stdout = TRUE,
    stderr = TRUE
  )
  
  # Parse output
  output <- paste(result$stdout, collapse = "\n")
  
  if (nchar(output) > 0) {
    tryCatch({
      response <- fromJSON(output)
    }, error = function(e) {
      response <- list(
        status = "error",
        message = paste("Failed to parse JSON:", e$message),
        raw_output = output
      )
    })
  } else {
    response <- list(status = "error", message = "No output from COMPASS")
  }
  
  # Add execution details
  response$return_code <- attr(result, "status")
  if (!is.null(result$stderr) && length(result$stderr) > 0) {
    response$stderr <- result$stderr
  }
  
  return(response)
}

# Example usage
config <- list(
  constraints = "data/BlockLand/artifical_cencus.csv",
  microdata   = "data/BlockLand/artifical_hh_survay.csv",
  groups      = "data/BlockLand/artificial_groups.csv",
  output      = "results/artificial_synthetic_population.csv",
  validate    = "results/artificial_synthPopSurvey.csv",
  initialTemp = 1000.0,
  minTemp = 0.001,
  coolingRate = 0.997,
  reheatFactor = 0.3,
  fitnessThreshold = 0.01,
  minImprovement = 0.001,
  maxIterations = 500000,
  windowSize = 5000,
  change = 100000,
  distance = "NORM_EUCLIDEAN",
  useRandomSeed = "no",
  randomSeed = 42
)

result <- run_compass(config)
cat("Status:", result$status, "\n")
cat("Message:", result$message, "\n")

if (!is.null(result$log)) {
  cat("\nExecution Log:\n")
  for (line in result$log) {
    cat(" ", line, "\n")
  }
}
```

---

## Configuration

### Required Parameters

| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `constraints` | string | Path to census constraint CSV file | `"data/census2021.csv"` |
| `groups` | string | Path to geographical grouping CSV file | `"data/groups.csv"` |
| `microdata` | string | Path to survey microdata CSV file | `"data/us_survey.csv"` |
| `output` | string | Path for output synthetic population CSV | `"results/population.csv"` |

### Optional Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `validate` | `""` | Path for validation output CSV (empty = no validation) |
| `initialTemp` | `1000.0` | Starting temperature for simulated annealing |
| `minTemp` | `0.001` | Minimum temperature before stopping |
| `coolingRate` | `0.997` | Temperature reduction factor per iteration |
| `reheatFactor` | `0.3` | Temperature increase when stagnation detected |
| `fitnessThreshold` | `0.01` | Target fitness value for early stopping |
| `minImprovement` | `0.001` | Minimum improvement threshold |
| `maxIterations` | `500000` | Maximum number of iterations per area |
| `windowSize` | `5000` | Iteration window for improvement checking |
| `change` | `100000` | Maximum rejected changes before stopping |
| `distance` | `"NORM_EUCLIDEAN"` | Distance metric (see below) |
| `useRandomSeed` | `"no"` | Use fixed random seed (`"yes"`/`"no"`) |
| `randomSeed` | `42` | Random seed if `useRandomSeed="yes"` |

### Input File Formats

#### Constraints CSV
```
id,total,var1,var2,var3,...
A001,100,25,40,35,...
A002,150,60,50,40,...
```

#### Microdata CSV
```
id,var1,var2,var3,...
P001,1,0,1,...
P002,0,1,1,...
```

#### Groups CSV
```
variable,group
var1,1
var2,2
var3,1
```

---

## Distance Metrics

The `distance` parameter determines how the algorithm measures the difference between the synthetic population and the constraints.

| Metric | Description | Best Used For |
|--------|-------------|---------------|
| **`EUCLIDEAN`** | Standard Euclidean distance (L2 norm) | Well-scaled data, physical distances |
| **`NORM_EUCLIDEAN`** | Normalized by target values | Mixed-scale attributes, percentage differences |
| **`COSINE`** | Angle between vectors | High-dimensional spaces, direction similarity |
| **`MANHATTEN`** | Sum of absolute differences (L1 norm) | Grid-like problems, robust to outliers |
| **`MSE`** | Mean Squared Error | Statistical fitting, regression problems |
| **`KLDivergence`** | Kullback-Leibler divergence | Probability distributions, information loss |
| **`CHI_SQUARED`** | Chi-squared distance | Goodness of fit comparisons |
| **`JSDIVERGENCE`** | Jensen-Shannon divergence | Symmetric probability distribution comparison |

### Metric Behavior

| Metric | Scale | Sensitivity | Best For |
|--------|-------|-------------|----------|
| EUCLIDEAN | Large values dominate | High for large errors | Physical measurements |
| NORM_EUCLIDEAN | Scale-invariant | Sensitive to near-zero | Mixed-scale data |
| COSINE | 0-2 range | Direction-based | High-dimensional data |
| MANHATTEN | Sum of absolute differences | Robust to outliers | Sparse data |
| MSE | Squared errors | Very sensitive to outliers | Statistical fitting |
| KLDivergence | Information-theoretic | Requires positive values | Distribution matching |

---

## Simulated Annealing Parameter Guide

### Temperature Parameters

| Parameter | Recommended Range | Effect |
|-----------|-------------------|--------|
| `initialTemp` | 500-2000 | Higher = more exploration early on |
| `minTemp` | 0.0001-0.01 | Lower = more precise convergence |
| `coolingRate` | 0.99-0.999 | Higher = slower cooling, better solutions |

### Convergence Control

| Parameter | Effect |
|-----------|--------|
| `fitnessThreshold` | Stop early if fitness target is met |
| `minImprovement` | Stop if improvement falls below this |
| `maxIterations` | Hard limit on iterations |

### Stagnation Handling

| Parameter | Effect |
|-----------|--------|
| `reheatFactor` | Percentage increase when stuck (0.3 = 30%) |
| `windowSize` | How many iterations to check for improvement |
| `change` | Stop if too many rejections in a row |

### Parameter Interplay Examples

**Quick Convergence Setup** (Fast results, "good enough" acceptable):
```json
{
  "initialTemp": 500.0,
  "coolingRate": 0.99,
  "fitnessThreshold": 0.05,
  "maxIterations": 100000,
  "reheatFactor": 0.5
}
```

**Thorough Search Setup** (Best possible solution, more time):
```json
{
  "initialTemp": 2000.0,
  "coolingRate": 0.998, 
  "fitnessThreshold": 0.001,
  "maxIterations": 2000000,
  "reheatFactor": 0.2
}
```

**Exploration-Focused Setup** (Escape local optima):
```json
{
  "initialTemp": 5000.0,
  "coolingRate": 0.999,
  "reheatFactor": 0.8,
  "windowSize": 10000
}
```

---

## Output Files

### Primary Outputs

1. **Synthetic Population CSV** (`output` parameter)
   - Maps geographical areas to synthetic individual IDs
   - Format: `area_id,microdata_id`

2. **Fraction Comparisons CSV** (`validate` parameter)
   - Shows how well constraints are matched
   - Format: `geography_code,variable,synth_fraction,constraint_fraction`

### Execution Results

The tool returns a JSON object with:
```json
{
  "status": "ok" | "error",
  "message": "Description of result",
  "log": ["Line 1", "Line 2", ...]
}
```

---

## Validation and Visualization

A Jupyter notebook called `CheckResults.ipynb` is included to visualize COMPASS's performance. It plots the predicted fractional values for each grouped variable alongside the corresponding synthetic fractions.

**Requirements:**
- Groups file (e.g., `data/BlockWorld/artificial_groups.csv`)
- Results file (e.g., `results/artificial_synthPopSurvey.csv`)

---

## Complete Example Configuration

```json
{
  "constraints": "data/BlockLand/artifical_cencus.csv",
  "microdata": "data/BlockLand/artifical_hh_survay.csv",
  "groups": "data/BlockLand/artificial_groups.csv",
  "output": "results/artificial_synthetic_population.csv",
  "validate": "results/artificial_synthPopSurvey.csv",
  "initialTemp": 1000.0,
  "minTemp": 0.001,
  "coolingRate": 0.997,
  "reheatFactor": 0.3,
  "fitnessThreshold": 0.01,
  "minImprovement": 0.001,
  "maxIterations": 500000,
  "windowSize": 5000,
  "change": 100000,
  "distance": "NORM_EUCLIDEAN",
  "useRandomSeed": "no",
  "randomSeed": 42
}
```

---

## Troubleshooting

### Common Issues

| Issue | Likely Cause | Solution |
|-------|--------------|----------|
| **"Failed to read constraints CSV"** | File path incorrect or format invalid | Check file exists, verify CSV format |
| **"Header mismatch"** | CSV column order differs | Ensure constraints, microdata, and groups headers align |
| **"No valid microdata found"** | Constraints don't match any records | Check zero-constraint rules, adjust constraints |
| **No improvement after many iterations** | Stuck in local optimum | Increase `reheatFactor` (0.5+), increase `initialTemp` |
| **Slow execution** | Too many iterations or large data | Decrease `maxIterations`, increase `fitnessThreshold` |
| **Poor constraint matching** | Cooling too fast | Decrease `coolingRate` (0.99), increase `maxIterations` |

### Log Messages

| Message | Meaning |
|---------|---------|
| `Loaded X constraint areas` | Successfully loaded input data |
| `Temperature reheated from X to Y` | Stagnation detected, reheating to escape local optimum |
| `Converged with fitness X <= threshold Y` | Successfully achieved target fitness |
| `No valid microdata found` | No records match zero-constraint rules |

### Performance Tips

1. **Start with smaller data**: Test with a subset of constraints/microdata
2. **Increase fitness threshold**: Higher threshold = faster convergence
3. **Reduce max iterations**: Lower for testing, higher for production
4. **Use appropriate distance metric**: `NORM_EUCLIDEAN` is good for mixed scales
5. **Enable deterministic mode**: Set `useRandomSeed: "yes"` for reproducible testing

---

## How It Works (Detailed)

### 1. Input Validation
All inputs are validated once at startup:
- Constraint values must match microdata dimensions
- Zero constraints enforced (if constraint is 0, microdata must be 0)
- Weights length must match constraint length

### 2. Population Initialization
For each area:
- Calculate population size (rounded from constraint.Total)
- Identify valid microdata records (satisfy zero constraints)
- Randomly select N records with replacement
- Compute initial aggregate totals

### 3. Simulated Annealing Optimization

For each iteration:
1. **Propose Change**: Randomly select:
   - A position in the population to replace
   - A valid microdata record to insert
2. **Evaluate**: Compute new aggregate totals and fitness
3. **Metropolis Criterion**:
   - If improvement (delta < 0): Accept
   - If equal (delta == 0): Accept (explore flat terrain)
   - If worse (delta > 0): Accept with probability e^(-delta/temp)
4. **Temperature Schedule**:
   - Cool: temp *= coolingRate
   - Reheat: temp *= (1 + reheatFactor) if stagnation detected
5. **Stagnation Detection**:
   - Monitor relative improvement over rolling window
   - Reheat if improvement falls below threshold
   - Exit if complete stagnation

### 4. Convergence Criteria

The algorithm stops when any of these are true:
- Best fitness ≤ fitnessThreshold
- maxIterations reached
- temperature ≤ minTemp
- changes ≤ 0 (all moves rejected)
- complete stagnation detected

### 5. Deterministic Mode

When `useRandomSeed: "yes"`:
- Each area gets a deterministic RNG seeded by `hash(areaID) + randomSeed`
- Results are reproducible across runs regardless of worker scheduling
- Uses FNV-1a hash for stable area ID hashing

---

## Development Status

**Version**: 0.80 (Experimental)

| Component | Status |
|-----------|--------|
| Core SA Algorithm | ✅ Stable |
| Distance Metrics | ✅ Complete |
| Parallel Processing | ✅ Stable |
| Deterministic Mode | ✅ Stable |
| Python Interface | ✅ Stable |
| R Interface | ✅ Stable |
| Linux Build | ✅ Stable  |
| Windows Build |✅ Stable  |
| macOS Build | ✅ Stable  |

---


## Acknowledgments

This project was developed with contributions from:

- **Core Algorithm Development**: Alison Heppenstall, Ricardo Colasanti, Hugh Rice, Andreas Hoehn 
- **Initial Project Design**: Nik Lomax Alison Heppenstall
- **Documentation and Code Comments**: Comprehensive documentation, code commenting, and technical writing assistance provided by **AI Assistant** through DeepSeek AI.


The simulated annealing implementation, distance metrics, parallel processing architecture, and cross-platform build scripts were documented and commented with the assistance of AI to ensure clarity, maintainability, and usability for researchers and developers.

---

*Documentation last updated: August 2026*

## License

This code is part of the COMPASS project. See the main project license for terms of use.

---

*Documentation last updated for version 0.80*  
*Tool under active development - Parameters and features may change*