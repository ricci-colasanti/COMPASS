# COMPASS - UK Spatial Synthetic Population Generator  
**Work in Progress - Not Ready for Production Use**  

*This is a research/experimental project for generating UK spatial synthetic populations.  
The code is under active development and not yet intended for public release or real-world use.*

---

## Overview

A Go-based tool for generating synthetic populations by combining UK census data with Understanding Society survey data using simulated annealing optimization. Creates spatially detailed synthetic populations that match statistical constraints from census data while preserving individual characteristics from survey microdata.

## Features

- **Parallel processing** - Utilizes all CPU cores for fast population generation
- **Multiple distance metrics** - KL Divergence, Chi-Squared, Euclidean, and more
- **Simulated annealing** - Intelligent optimization algorithm to match constraints
- **Validation outputs** - Generates comparison files to verify constraint matching
- **Multi-language API** - Call from Python, R, or directly via command line
- **UK-focused** - Designed for UK census geography and Understanding Society data

## Installation
Pre-compiled Binaries

Pre-compiled executables are available for:

    Linux (64-bit): Included in the repository

    Windows: Coming soon (currently build from source)

    macOS: Coming soon (currently build from source)

Download the appropriate binary for your system or build from source.
Prerequisites (for Building from Source)

    Go 1.18+ (only required if building from source)

    Python 3.6+ (for Python interface - optional)

    R 4.0+ with jsonlite package (for R interface - optional)

Quick Start (Using Pre-compiled Binary)

    For Linux users (64-bit):
    bash

# Download the Linux binary
chmod +x compass

# Test it works
./compass --help

# Building from Source

If pre-compiled binaries don't work for your system, or you want the latest development version:
bash

# Clone the repository
git clone <repository-url>
cd compass

# Build the binary
go build -o compass main.go

# Make it executable (Linux/macOS)
chmod +x compass

# Test the build
./compass --version

Note for Windows users: When building from source on Windows, the output will be compass.exe. Use go build -o compass.exe main.go and run with compass.exe instead of ./compass.

## Usage

### Command Line (Direct JSON Input)
```bash
# Method 1: Pipe JSON to stdin
echo '{"constraints":"data.csv","microdata":"micro.csv"}' | ./compass

# Method 2: JSON file input
./compass < config.json

# Method 3: Direct parameters (simple mode)
./compass --constraints data.csv --microdata micro.csv --output results.csv
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

## Check results
Included in the files is a Jupyter notebook called CheckResults.ipynb. This visualizes COMPASS’s performance by plotting the predicted fractional values for each grouped variable alongside the corresponding synthetic fractions generated by COMPASS. It needs a groups file and a results file. In the demo these are "data/BlockWorld/artificial_groups.csv" and "results/artificial_synthPopSurvey.csv.

## Configuration Parameters

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
| `initialTemp` | 1000.0 | Starting temperature for simulated annealing |
| `minTemp` | 0.001 | Minimum temperature before stopping |
| `coolingRate` | 0.997 | Temperature reduction factor per iteration |
| `reheatFactor` | 0.3 | Temperature increase when stagnation detected |
| `fitnessThreshold` | 0.01 | Target fitness value for early stopping |
| `minImprovement` | 0.001 | Minimum improvement threshold |
| `maxIterations` | 500000 | Maximum number of iterations |
| `windowSize` | 5000 | Iteration window for improvement checking |
| `change` | 100000 | Maximum accepted changes before stopping |
| `distance` | `"NORM_EUCLIDEAN"` | Distance metric (see below) |
| `useRandomSeed` | `"no"` | Use fixed random seed (`"yes"`/`"no"`) |
| `randomSeed` | 42 | Random seed if `useRandomSeed="yes"` |

### **Distance Metric** 
- `"EUCLIDEAN"`: Standard Euclidean distance
- `"NORM_EUCLIDEAN"`: Normalized Euclidean (default, good for mixed scales)
- `"COSINE"`: Cosine similarity (angle-based)
- `"MANHATTEN"`: Manhattan (taxicab) distance
- `"MSE"`: Mean Squared Error
- `"KLDivergence"`: Kullback-Leibler Divergence




#### `distance`: "NORM_EUCLIDEAN"
**What it does**: How to measure difference between target and current solution
**Behavior impact**:

**"EUCLIDEAN"** - Standard distance:
- Good for: Well-scaled data, physical distances
- Watch out: Large-value attributes dominate

**"NORM_EUCLIDEAN"** - Normalized by target values:
- Good for: Mixed-scale attributes, percentage differences
- Watch out: Sensitive to near-zero constraints

**"COSINE"** - Angle between vectors:  
- Good for: Direction similarity, high-dimensional spaces
- Watch out: Ignores magnitude differences

**"MANHATTEN"** - Sum of absolute differences:
- Good for: Grid-like problems, robust to outliers
- Watch out: Less sensitive to large errors

**"MSE"** - Mean Squared Error:
- Good for: Statistical fitting, regression problems
- Watch out: Very small values, needs careful thresholds

**"KLDivergence"** - Information theory distance:
- Good for: Probability distributions, information loss
- Watch out: Asymmetric, requires positive values

## **Parameter Interplay Examples**

### **Quick Convergence Setup**
```json
{
  "initialTemp": 500.0,
  "coolingRate": 0.99,
  "fitnessThreshold": 0.05,
  "maxIterations": 100000,
  "reheatFactor": 0.5
}
```
*Use when: You need fast results and "good enough" is acceptable*

### **Thorough Search Setup**  
```json
{
  "initialTemp": 2000.0,
  "coolingRate": 0.998, 
  "fitnessThreshold": 0.001,
  "maxIterations": 2000000,
  "reheatFactor": 0.2
}
```
*Use when: You need the best possible solution and have time*

### **Exploration-Focused Setup**
```json
{
  "initialTemp": 5000.0,
  "coolingRate": 0.999,
  "reheatFactor": 0.8,
  "windowSize": 10000
}
```
## Output Files

### Primary Outputs
1. **Synthetic Population CSV** (`output` parameter)
   - Maps geographical areas to synthetic individuals
   - Format: `area_id,microdata_id` rows

2. **Fraction Comparisons CSV** (if validation enabled)
   - Shows how well constraints are matched
   - Format: `geography_code,variable,synth_fraction,constraint_fraction`

### Execution Results
The tool returns a JSON object with:
```json
{
  "status": "ok" | "error",
  "message": "Description of result",
  "iterations": 12345,
  "final_fitness": 0.005,
  "log": ["Line 1", "Line 2", ...]
}
```

## Simulated Annealing Parameters Guide

### Temperature Parameters 🌡️

| Parameter | Default | Recommended Range | Description |
|-----------|---------|-------------------|-------------|
| `initialTemp` | 1000.0 | 500-2000 | Starting temperature for annealing |
| `minTemp` | 0.001 | 0.0001-0.01 | Minimum temperature before stopping |
| `coolingRate` | 0.997 | 0.99-0.999 | Temperature reduction per iteration |

### Convergence Control 

| Parameter | Default | Description |
|-----------|---------|-------------|
| `fitnessThreshold` | 0.01 | Stop when fitness reaches this value |
| `minImprovement` | 0.001 | Minimum improvement over window to continue |
| `maxIterations` | 500000 | Absolute maximum iteration limit |

### Stagnation Handling 

| Parameter | Default | Description |
|-----------|---------|-------------|
| `reheatFactor` | 0.3 | Temperature increase when stagnation detected |
| `windowSize` | 5000 | Iterations to check for improvement |
| `change` | 100000 | Maximum accepted changes before stopping |

## Example Configurations

{
  "constraints" : "data/BlockLand/artifical_cencus.csv",
  "microdata"   : "data/BlockLand/artifical_hh_survay.csv",
  "groups"      : "data/BlockLand/artificial_groups.csv",
  "output"      : "results/artificial_synthetic_population.csv",
  "validate"    : "results/artificial_synthPopSurvey.csv",
  "initialTemp"      : 1000.0,
  "minTemp"          : 0.001,
  "coolingRate"      : 0.997,
  "reheatFactor"     : 0.3,
  "fitnessThreshold" : 0.01,
  "minImprovement"   : 0.001,
  "maxIterations"    : 500000,
  "windowSize"       : 5000,
  "change"           : 100000,
  "distance"         : "NORM_EUCLIDEAN",
  "useRandomSeed"    : "no",
  "randomSeed"       : 42
}

```

## Troubleshooting

### Common Issues

**Error**: "Failed to read constraints CSV"
- Check file path exists and is readable
- Verify CSV format matches expected structure
- Ensure proper comma separation and quoting

**Error**: "No improvement after X iterations"
- Increase `reheatFactor` (0.5+)
- Increase `initialTemp` (2000+)
- Try different `distance` metric

**Issue**: Running very slowly
- Decrease `maxIterations` (100000)
- Increase `fitnessThreshold` (0.05)
- Reduce data size for testing

**Issue**: Poor constraint matching
- Decrease `coolingRate` (0.99)
- Increase `maxIterations` (500000+)
- Use `"NORM_EUCLIDEAN"` distance metric

### Log Messages

- `"INFO: Loaded X constraint areas"` - Normal loading message
- `"WARN: Temperature reheated from X to Y"` - Stagnation detected and handled
- `"ERROR: Failed to read file"` - File I/O issue, check paths and permissions
- `"STATUS: Converged after X iterations"` - Successful completion

## How It Works

1. **Input Loading**: Reads constraint data, microdata, and geographical groupings
2. **Initialization**: Creates random population matching microdata distributions
3. **Annealing Loop**: Iteratively improves population to match constraints:
   - Proposes changes (swapping individuals)
   - Accepts/rejects based on fitness improvement and temperature
   - Cools temperature over time
   - Reheats if stuck in local minimum
4. **Output Generation**: Writes final population and optional validation data

## Development Status

**Version**: 0.80 (Experimental)


---

*This documentation last updated for version 0.41*  
*Tool under active development - Parameters and features may change*
