package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"golang.org/x/term"
)

// ============================================================================
// CONFIGURATION AND RESULT STRUCTS
// ============================================================================

// Config represents the entire user‑supplied configuration loaded from JSON.
// It contains file paths, annealing hyperparameters, and a flag for random seed.
type Config struct {
	Constraints      string  `json:"constraints"`      // path to constraints CSV
	Groups           string  `json:"groups"`           // path to grouping CSV
	Microdata        string  `json:"microdata"`        // path to microdata CSV
	Output           string  `json:"output"`           // path for IDs output
	Validate         string  `json:"validate"`         // path for fractions output
	InitialTemp      float64 `json:"initialTemp"`      // starting temperature
	MinTemp          float64 `json:"minTemp"`          // minimum temperature
	CoolingRate      float64 `json:"coolingRate"`      // cooling factor (0..1)
	ReheatFactor     float64 `json:"reheatFactor"`     // percentage increase on stagnation
	FitnessThreshold float64 `json:"fitnessThreshold"` // early stop if fitness <= this
	MinImprovement   float64 `json:"minImprovement"`   // relative improvement threshold
	MaxIterations    int     `json:"maxIterations"`    // maximum SA iterations per area
	WindowSize       int     `json:"windowSize"`       // rolling window size for stagnation
	Change           int     `json:"change"`           // max rejected moves before stop
	Distance         string  `json:"distance"`         // distance metric name
	UseRandomSeed    string  `json:"useRandomSeed"`    // "yes" or "no"
	RandomSeed       int     `json:"randomSeed"`       // seed value (used if UseRandomSeed=="yes")
}

// AnnealingConfig is a subset of Config used internally by the SA algorithm.
// It holds only the parameters that affect the annealing process.
type AnnealingConfig struct {
	InitialTemp      float64 `json:"initialTemp"`
	MinTemp          float64 `json:"minTemp"`
	CoolingRate      float64 `json:"coolingRate"`
	ReheatFactor     float64 `json:"reheatFactor"`
	FitnessThreshold float64 `json:"fitnessThreshold"`
	MinImprovement   float64 `json:"minImprovement"`
	MaxIterations    int     `json:"maxIterations"`
	WindowSize       int     `json:"windowSize"`
	Change           int     `json:"change"`
	Distance         string  `json:"distance"`
	UseRandomSeed    string  `json:"useRandomSeed"`
	RandomSeed       int     `json:"randomSeed,omitempty"` // optional
}

// WeightsData is a placeholder for future weighted microdata.
type WeightsData struct {
	ID     string
	Values []float64
}

// MicroData represents a single survey record with an ID and a vector of values.
type MicroData struct {
	ID     string
	Values []float64
}

// ConstraintData represents an area's target totals and its total population.
type ConstraintData struct {
	ID     string
	Values []float64 // totals for each variable
	Total  float64   // total population
}

// results holds the output of a single area's synthetic population.
type results struct {
	area              string
	population        float64
	synthpop_totals   []float64 // aggregate totals of the synthetic population
	ids               []string  // IDs of selected microdata records
	constraint_totals []float64 // original constraint values
	fitness           float64   // final fitness (MSE)
}

// UIUpdate is used to send progress messages from the parallel engine to the UI/logger.
type UIUpdate struct {
	Text    string    // progress text
	Fitness []float64 // current fitness values (for monitoring)
}

// ============================================================================
// VALID METRICS (documentation only)
// ============================================================================

// ValidMetrics lists all recognised distance metric names.
// This is used for validation and help messages.
var ValidMetrics = []string{
	"CHI_SQUARED", "EUCLIDEAN", "NORM_EUCLIDEAN",
	"MANHATTEN", "KL_DIVERGENCE", "COSINE", "JSDIVERGENCE",
}

// ============================================================================
// POPULATION CONFIG (legacy/alternative config layout)
// ============================================================================

// PopulationConfig is an alternative, nested JSON structure for configuration.
// It is not currently used but kept for compatibility.
type PopulationConfig struct {
	Constraints struct {
		File string `json:"file"`
	} `json:"constraints"`
	Microdata struct {
		File string `json:"file"`
	} `json:"microdata"`
	Weights struct {
		UseWeights string
		File       string `json:"file"`
	} `json:"weights"`
	Output struct {
		File string `json:"file"`
	} `json:"output"`
	Validate struct {
		File string `json:"file"`
	} `json:"validate"`
}

// ============================================================================
// LOGGING INFRASTRUCTURE
// ============================================================================

var (
	// logBuf is an in‑memory buffer that captures all log output.
	// It is used to include logs in the final JSON response when in R mode.
	logBuf bytes.Buffer

	// logger is the global logger. It writes to both logBuf and (in CLI mode) stderr.
	logger *log.Logger
)

// rMode returns true when stdout is not a terminal.
// This indicates that the program is being run in a pipe (e.g., from R or another process)
// and that we should output JSON only and include the log in the response.
func rMode() bool {
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// initLogger configures the logger once per run.
// It always writes to the in‑memory buffer. In CLI mode (non‑R mode), it also
// duplicates output to stderr so the user can see progress in real time.
func initLogger(isR bool) {
	writers := []io.Writer{&logBuf}
	if !isR { // CLI mode – echo to stderr
		writers = append(writers, os.Stderr)
	}
	logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
}

// infof and info are thin wrappers around logger.Printf and logger.Print.
// They replace fmt.Printf/Println throughout the code to ensure all output
// goes through the central logging system.
func infof(format string, a ...any) { logger.Printf(format, a...) }
func info(a ...any)                 { logger.Print(a...) }

// ============================================================================
// JSON RESPONSE STRUCTURES AND EMITTERS
// ============================================================================

// Resp defines the structure of the JSON response sent to stdout.
// It contains a status, an optional message, and an optional log array.
type Resp struct {
	Status  string   `json:"status"`            // "ok", "error", "gui", "default"
	Message string   `json:"message,omitempty"` // human-readable message
	Log     []string `json:"log,omitempty"`     // captured log lines (only in R mode)
}

// emitResponse writes a JSON response to stdout. If isR is true, it includes
// the accumulated log lines. It then resets the log buffer.
func emitResponse(status, msg string, isR bool) {
	var lines []string
	if isR && logBuf.Len() > 0 {
		// Split the buffer into lines and trim trailing newline.
		lines = strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	}
	_ = json.NewEncoder(os.Stdout).Encode(Resp{
		Status:  status,
		Message: msg,
		Log:     lines,
	})
	// Reset buffer for the next run (useful if the binary stays alive).
	logBuf.Reset()
}

// emitError is a helper that sends an error response with a standard format.
// It appends a reminder of the expected JSON schema to the error message.
func emitError(msg string) {
	expectedJSON := `{  constraints:string,microdata:string,output:string,validate:string,initialTemp:float64,minTemp:float64,coolingRate:float64,reheatFactor:float64,fitnessThreshold:float64,minImprovement:float64,maxIterations:int, windowSize:int,change:int,distance:string,useRandomSeed:yes | no,randomSeed:int}`
	fullMsg := fmt.Sprintf("%s Expected JSON format:%s", msg, expectedJSON)
	emitResponse("error", fullMsg, rMode())
}

// ============================================================================
// CORE HELPERS (JSON decode, CSV loaders, etc.)
// ============================================================================

// decodeJSON reads a JSON configuration from an io.Reader and returns a Config.
// It disallows unknown fields to catch typos in the configuration file.
func decodeJSON(src io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(src)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// loadGroups reads the grouping CSV file and returns a slice of group IDs and the header.
// The grouping defines which variable belongs to which aggregate group (e.g., age groups).
func loadGroups(groupsFile string) ([]int, []string, error) {
	groups, header, err := ReadGroupingCSV(groupsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(groups))
	return groups, header, nil
}

// loadConstraints reads the constraints CSV and returns a slice of ConstraintData.
func loadConstraints(constraintsFile string) ([]ConstraintData, []string, error) {
	constraints, header, err := ReadConstraintCSV(constraintsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(constraints))
	return constraints, header, nil
}

// loadMicrodata reads the microdata CSV and returns a slice of MicroData.
func loadMicrodata(microdataFile string) ([]MicroData, []string, error) {
	microData, header, err := ReadMicroDataCSV(microdataFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read microdata CSV: %w", err)
	}
	infof("Loaded %d microdata records", len(microData))
	return microData, header, nil
}

// loadInputData aggregates all input data from the configuration.
// It loads constraints, groups, microdata, and builds a uniform weight vector.
// If any file fails to load, it logs the error and exits the program.
//
// Returns:
//   - constraints, constraintHeader
//   - groups, groupsHeader
//   - microData, microDataHeader
//   - weights, weightsHeader (currently uniform 1.0 for all variables)
func loadInputData(config Config) ([]ConstraintData, []string, []int, []string, []MicroData, []string, []float64, []string) {
	constraints, constraintHeader, err := loadConstraints(config.Constraints)
	if err != nil {
		infof("Constraint loading error: %v", err)
		os.Exit(1)
	}

	groups, groupsHeader, err := loadGroups(config.Groups)
	if err != nil {
		infof("Groups loading error: %v", err)
		os.Exit(1)
	}

	microData, microDataHeader, err := loadMicrodata(config.Microdata)
	if err != nil {
		infof("Microdata loading error: %v", err)
		os.Exit(1)
	}

	// Build uniform weights (all 1.0) – can be customised later.
	weights := make([]float64, len(constraints[0].Values))
	for i := range weights {
		weights[i] = 1.0
	}

	// Create a header for weights (for documentation/debugging).
	weightsHeader := make([]string, len(weights))
	for i := range weightsHeader {
		weightsHeader[i] = fmt.Sprintf("Weight_%d", i+1)
	}

	return constraints, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader
}

// ============================================================================
// MAIN SIMULATION DRIVER
// ============================================================================

// runMicrosim orchestrates the entire simulation:
//   - initialises the logger
//   - loads all input data
//   - validates that headers match
//   - sets up a channel for UI updates
//   - builds the annealing config
//   - calls parallelRun to process all areas
//   - emits a final JSON response
//
// It exits with code 1 on fatal errors.
func runMicrosim(config Config) {
	isR := rMode()
	initLogger(isR) // set up logger once for the whole run

	// Load all input data.
	constraintData, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader := loadInputData(config)

	// ----------------------------------------------------------------------
	// Header validation: ensure constraints header matches microdata header
	// and also matches groups and weights headers if they exist.
	// If they don't match, log an error and exit.
	// ----------------------------------------------------------------------
	if (!reflect.DeepEqual(constraintHeader, microDataHeader)) &&
		(!reflect.DeepEqual(constraintHeader, weightsHeader)) &&
		(!reflect.DeepEqual(constraintHeader, groupsHeader)) {

		info("Error: The Constraints header and the MicroData or the Groups headers are not the same")
		for i := 0; i < len(constraintHeader); i++ {
			infof("%s %s %t", constraintHeader[i], microDataHeader[i],
				microDataHeader[i] == constraintHeader[i])
		}
		emitResponse("error", "header mismatch", isR)
		os.Exit(1)
	}

	info("Running in command-line mode...")
	start := time.Now()

	// ----------------------------------------------------------------------
	// UI updates channel – the parallel engine sends progress messages here.
	// A separate goroutine consumes them and logs them via the logger.
	// ----------------------------------------------------------------------
	uiUpdates := make(chan UIUpdate, 10)
	go func() {
		for upd := range uiUpdates {
			info(upd.Text)
		}
	}()

	// Build the annealing configuration structure from the flat config.
	annealingConfig := AnnealingConfig{
		InitialTemp:      config.InitialTemp,
		MinTemp:          config.MinTemp,
		CoolingRate:      config.CoolingRate,
		ReheatFactor:     config.ReheatFactor,
		FitnessThreshold: config.FitnessThreshold,
		MinImprovement:   config.MinImprovement,
		MaxIterations:    config.MaxIterations,
		WindowSize:       config.WindowSize,
		Change:           config.Change,
		Distance:         config.Distance,
		UseRandomSeed:    config.UseRandomSeed,
		RandomSeed:       config.RandomSeed,
	}

	// ----------------------------------------------------------------------
	// Run the core parallel simulated annealing engine.
	// ----------------------------------------------------------------------
	parallelRun(constraintData, groups, microData, weights, microDataHeader,
		config.Output, config.Validate, annealingConfig, uiUpdates)

	elapsed := time.Since(start)
	infof("Completed in %s", elapsed)

	// Close the UI updates channel (the consumer goroutine will exit).
	close(uiUpdates)

	// ----------------------------------------------------------------------
	// Send final JSON response.
	// ----------------------------------------------------------------------
	emitResponse("ok", "simulation finished", isR)
}

// ============================================================================
// ENTRY POINT
// ============================================================================

// main parses command‑line flags, reads the JSON configuration (either from
// a file or stdin), and starts the simulation.
//
// Flags:
//
//	-f <file> : path to a JSON config file
//	-g        : GUI mode (placeholder)
//
// If no -f is given and stdin is not a terminal, it reads JSON from stdin.
// If stdin is a terminal and no -f is given, it prints a help message and exits.
func main() {
	// ----- Command‑line flag parsing -------------------------------------
	filePath := flag.String("f", "", "path to a JSON config file")
	guiFlag := flag.Bool("g", false, "open the GUI (placeholder)")
	flag.Parse()

	// ----- GUI flag (not implemented) ------------------------------------
	if *guiFlag {
		fmt.Fprintln(os.Stderr, " opening GUI … (placeholder)")
		emitResponse("gui", "GUI would be launched here", rMode())
		return
	}

	// ----- Determine input source ---------------------------------------
	var src io.Reader
	if *filePath != "" {
		// Read configuration from the specified file.
		f, err := os.Open(*filePath)
		if err != nil {
			emitResponse("error", "cannot open file: "+err.Error(), rMode())
			os.Exit(1)
		}
		defer f.Close()
		src = f
		fmt.Fprintln(os.Stderr, "loading config from:", *filePath)
	} else {
		// No -f flag: check if stdin has data (pipe or redirection).
		if term.IsTerminal(int(os.Stdin.Fd())) {
			// Interactive terminal with no input – show usage.
			fmt.Fprintln(os.Stderr, "  No input on stdin – use -f <file>.json or pipe JSON")
			emitResponse("default", "no input provided", rMode())
			return
		}
		// Read from stdin.
		src = os.Stdin
	}

	// ----- Decode the JSON configuration --------------------------------
	cfg, err := decodeJSON(src)
	if err != nil {
		emitError("invalid JSON: " + err.Error())
		os.Exit(1)
	}

	// ----- Run the simulation -------------------------------------------
	runMicrosim(cfg)
}
