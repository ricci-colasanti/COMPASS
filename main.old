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

/*
==============================================================
====  CONFIG & RESULT STRUCTS
==============================================================
*/
type Config struct {
	Constraints      string  `json:"constraints"`
	Groups           string  `json:"groups"`
	Microdata        string  `json:"microdata"`
	Output           string  `json:"output"`
	Validate         string  `json:"validate"`
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
	UseRandomSeed    string  `json:"useRandomSeed"` // "yes"/"no"
	RandomSeed       int     `json:"randomSeed"`    // integer seed
}

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

type WeightsData struct {
	ID     string
	Values []float64
}
type MicroData struct {
	ID     string
	Values []float64
}
type ConstraintData struct {
	ID     string
	Values []float64
	Total  float64
}
type results struct {
	area              string
	population        float64
	synthpop_totals   []float64
	ids               []string
	constraint_totals []float64
	fitness           float64
}

/* UIUpdate – messages that the core algorithm can push back */
type UIUpdate struct {
	Text    string
	Fitness []float64
}

/*
-----------------------------------------------------------------

	VALID METRICS

-----------------------------------------------------------------
*/
var ValidMetrics = []string{
	"CHI_SQUARED", "EUCLIDEAN", "NORM_EUCLIDEAN",
	"MANHATTEN", "KL_DIVERGENCE", "COSINE", "JSDIVERGENCE",
}

/*
-----------------------------------------------------------------

	POPULATION CONFIG

-----------------------------------------------------------------
*/
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

/*
==============================================================
====  LOGGING INFRASTRUCTURE
==============================================================
*/
var (
	logBuf bytes.Buffer
	logger *log.Logger
)

/*
rMode returns true when stdout is NOT a terminal.

That is the case when R (or any other pipe) captures the output.
*/
func rMode() bool {
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

/*
Initialise the logger once per run.
  - Always write to the in‑memory buffer.
  - When we are in CLI mode also duplicate to stderr so the user sees it.
*/
func initLogger(isR bool) {
	writers := []io.Writer{&logBuf}
	if !isR { // CLI → also echo to stderr
		writers = append(writers, os.Stderr)
	}
	logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
}

/* Thin wrappers that replace fmt.Printf/Println throughout the code */
func infof(format string, a ...any) { logger.Printf(format, a...) }
func info(a ...any)                 { logger.Print(a...) }

/*
--------------------------------------------------------------

	JSON RESPONSE STRUCT

--------------------------------------------------------------
*/
type Resp struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Log     []string `json:"log,omitempty"`
}

/*
Emit the final JSON payload.
If in R‑mode also attach the accumulated log lines.
*/
func emitResponse(status, msg string, isR bool) {
	var lines []string
	if isR && logBuf.Len() > 0 {
		lines = strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	}
	_ = json.NewEncoder(os.Stdout).Encode(Resp{
		Status:  status,
		Message: msg,
		Log:     lines,
	})
	// Reset the buffer for the next run (good practice if the binary stays alive)
	logBuf.Reset()
}

func emitError(msg string) {
	expectedJSON := `{  constraints:string,microdata:string,output:string,validate:string,initialTemp:float64,minTemp:float64,coolingRate:float64,reheatFactor:float64,fitnessThreshold:float64,minImprovement:float64,maxIterations:int, windowSize:int,change:int,distance:string,useRandomSeed:yes | no,randomSeed:int}`
	fullMsg := fmt.Sprintf("%s Expected JSON format:%s", msg, expectedJSON)
	emitResponse("error", fullMsg, rMode())
}

/*
==============================================================
====  CORE HELPERS (JSON decode, CSV loaders, etc.)
==============================================================
*/
func decodeJSON(src io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(src)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

/* Load constraints */
func loadGroups(groupsFile string) ([]int, []string, error) {
	groups, header, err := ReadGroupingCSV(groupsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(groups))
	return groups, header, nil
}

/* Load constraints */
func loadConstraints(constraintsFile string) ([]ConstraintData, []string, error) {
	constraints, header, err := ReadConstraintCSV(constraintsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(constraints))
	return constraints, header, nil
}

/* Load micro‑data */
func loadMicrodata(microdataFile string) ([]MicroData, []string, error) {
	microData, header, err := ReadMicroDataCSV(microdataFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read microdata CSV: %w", err)
	}
	infof("Loaded %d microdata records", len(microData))
	return microData, header, nil
}

/* Aggregate all input data */

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

	// Simple uniform weights – replace with real logic if needed
	weights := make([]float64, len(constraints[0].Values))
	for i := range weights {
		weights[i] = 1.0
	}

	// Create a proper header for weights
	// Assuming weights correspond to constraint values
	weightsHeader := make([]string, len(weights))
	for i := range weightsHeader {
		weightsHeader[i] = fmt.Sprintf("Weight_%d", i+1)
	}
	// OR if you want to use constraint column names:
	// weightsHeader = constraintHeader  // if they align

	return constraints, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader
}

/*
--------------------------------------------------------------

	MAIN simulation driver – logs via the unified logger

--------------------------------------------------------------
*/
func runMicrosim(config Config) {
	isR := rMode()
	initLogger(isR) // set up logger *once* for the whole run

	constraintData, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader := loadInputData(config)

	/* ----------- Header validation (log + JSON on error) ----------- */
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

	/* ---------------- UI‑updates channel ---------------- */
	uiUpdates := make(chan UIUpdate, 10)
	go func() {
		for upd := range uiUpdates {
			// Whatever the core algorithm sends gets logged
			info(upd.Text)
		}
	}()

	/* ---------------- Build annealing config ---------------- */
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

	/* ---------------- Run the core algorithm ---------------- */
	parallelRun(constraintData, groups, microData, weights, microDataHeader,
		config.Output, config.Validate, annealingConfig, uiUpdates)

	elapsed := time.Since(start)
	infof("Completed in %s", elapsed)

	close(uiUpdates)

	/* ---------------- Final JSON response ---------------- */
	emitResponse("ok", "simulation finished", isR)
}

/*
==============================================================
====  ENTRY POINT (main)
==============================================================
*/
func main() {
	// ----- flag handling -------------------------------------------------
	filePath := flag.String("f", "", "path to a JSON config file")
	guiFlag := flag.Bool("g", false, "open the GUI (placeholder)")
	flag.Parse()

	// ----- GUI flag -------------------------------------------------------
	if *guiFlag {
		fmt.Fprintln(os.Stderr, " opening GUI … (placeholder)")
		emitResponse("gui", "GUI would be launched here", rMode())
		return
	}

	// ----- Determine input source (file vs stdin) ------------------------
	var src io.Reader
	if *filePath != "" {
		f, err := os.Open(*filePath)
		if err != nil {
			emitResponse("error", "cannot open file: "+err.Error(), rMode())
			os.Exit(1)
		}
		defer f.Close()
		src = f
		fmt.Fprintln(os.Stderr, "loading config from:", *filePath)
	} else {
		// No -f flag – make sure stdin isn’t a terminal
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "  No input on stdin – use -f <file>.json or pipe JSON")
			emitResponse("default", "no input provided", rMode())
			return
		}
		src = os.Stdin
	}

	// ----- Decode the JSON -----------------------------------------------
	cfg, err := decodeJSON(src)
	if err != nil {
		emitError("invalid JSON: " + err.Error())
		os.Exit(1)
	}

	// ----- Run the simulation -------------------------------------------
	runMicrosim(cfg)

}
