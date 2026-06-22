package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
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
	UseRandomSeed    string  `json:"useRandomSeed"`
	RandomSeed       int     `json:"randomSeed"`
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
	RandomSeed       int     `json:"randomSeed,omitempty"`
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

type UIUpdate struct {
	Text    string
	Fitness []float64
}

var ValidMetrics = []string{
	"CHI_SQUARED", "EUCLIDEAN", "NORM_EUCLIDEAN",
	"MANHATTEN", "KL_DIVERGENCE", "COSINE", "JSDIVERGENCE",
}

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

func rMode() bool {
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

func initLogger(isR bool) {
	writers := []io.Writer{&logBuf}
	if !isR {
		writers = append(writers, os.Stderr)
	}
	logger = log.New(io.MultiWriter(writers...), "", log.LstdFlags)
}

func infof(format string, a ...any) { logger.Printf(format, a...) }
func info(a ...any)                 { logger.Print(a...) }

type Resp struct {
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	Log     []string `json:"log,omitempty"`
}

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
	logBuf.Reset()
}

func emitError(msg string) {
	expectedJSON := `{  constraints:string,microdata:string,output:string,validate:string,initialTemp:float64,minTemp:float64,coolingRate:float64,reheatFactor:float64,fitnessThreshold:float64,minImprovement:float64,maxIterations:int, windowSize:int,change:int,distance:string,useRandomSeed:yes | no,randomSeed:int}`
	fullMsg := fmt.Sprintf("%s Expected JSON format:%s", msg, expectedJSON)
	emitResponse("error", fullMsg, rMode())
}

func decodeJSON(src io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(src)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func loadGroups(groupsFile string) ([]int, []string, error) {
	groups, header, err := ReadGroupingCSV(groupsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(groups))
	return groups, header, nil
}

func loadConstraints(constraintsFile string) ([]ConstraintData, []string, error) {
	constraints, header, err := ReadConstraintCSV(constraintsFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read constraints CSV: %w", err)
	}
	infof("Loaded %d constraint areas", len(constraints))
	return constraints, header, nil
}

func loadMicrodata(microdataFile string) ([]MicroData, []string, error) {
	microData, header, err := ReadMicroDataCSV(microdataFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read microdata CSV: %w", err)
	}
	infof("Loaded %d microdata records", len(microData))
	return microData, header, nil
}

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

	weights := make([]float64, len(constraints[0].Values))
	for i := range weights {
		weights[i] = 1.0
	}

	weightsHeader := make([]string, len(weights))
	for i := range weightsHeader {
		weightsHeader[i] = fmt.Sprintf("Weight_%d", i+1)
	}

	return constraints, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader
}

func runMicrosim(config Config) {
	isR := rMode()
	initLogger(isR)

	constraintData, constraintHeader,
		groups, groupsHeader,
		microData, microDataHeader,
		weights, weightsHeader := loadInputData(config)

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

	uiUpdates := make(chan UIUpdate, 10)
	go func() {
		for upd := range uiUpdates {
			info(upd.Text)
		}
	}()

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

	parallelRun(constraintData, groups, microData, weights, microDataHeader,
		config.Output, config.Validate, annealingConfig, uiUpdates)

	elapsed := time.Since(start)
	infof("Completed in %s", elapsed)

	close(uiUpdates)
	emitResponse("ok", "simulation finished", isR)
}

/*
==============================================================
====  SIMPLIFIED GUI WITH PROPER THREADING
==============================================================
*/
type GUI struct {
	window      fyne.Window
	statusLabel *widget.Label
	logText     *widget.Entry
	configView  *widget.Entry
	configData  Config
	configPath  string
}

// Safe UI update functions using fyne.Do
func (g *GUI) updateStatus(text string) {
	fyne.Do(func() {
		g.statusLabel.SetText(text)
	})
}

func (g *GUI) appendLog(text string) {
	fyne.Do(func() {
		current := g.logText.Text
		if current != "" && !strings.HasSuffix(current, "\n") {
			current += "\n"
		}
		g.logText.SetText(current + text)
	})
}

func (g *GUI) setConfigText(text string) {
	fyne.Do(func() {
		g.configView.SetText(text)
	})
}

func runGUI() {
	myApp := app.NewWithID("com.compass.gui")
	myWindow := myApp.NewWindow("COMPASS GUI")

	gui := &GUI{
		window: myWindow,
	}

	// Status label
	gui.statusLabel = widget.NewLabel("Ready - Load a config file")

	// Log text area - use standard entry, not disabled
	gui.logText = widget.NewMultiLineEntry()
	gui.logText.SetPlaceHolder("Log output will appear here...")
	gui.logText.Wrapping = fyne.TextWrapBreak
	gui.logText.OnChanged = func(s string) {
		// Prevent user editing
	}
	gui.logText.TextStyle = fyne.TextStyle{}
	logScroll := container.NewScroll(gui.logText)
	logScroll.SetMinSize(fyne.NewSize(700, 300))

	// Config display - similar approach
	gui.configView = widget.NewMultiLineEntry()
	gui.configView.SetPlaceHolder("Load a config file to see its contents here...")
	gui.configView.Wrapping = fyne.TextWrapBreak
	gui.configView.TextStyle = fyne.TextStyle{}
	configScroll := container.NewScroll(gui.configView)
	configScroll.SetMinSize(fyne.NewSize(700, 200))

	// Load Config button - Blue
	loadConfigBtn := widget.NewButton("Load Config", func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			configData, err := io.ReadAll(reader)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to read config: %v", err), myWindow)
				return
			}

			var config Config
			if err := json.Unmarshal(configData, &config); err != nil {
				dialog.ShowError(fmt.Errorf("Failed to parse JSON: %v", err), myWindow)
				return
			}

			gui.configData = config
			gui.configPath = reader.URI().Path()

			prettyJSON, _ := json.MarshalIndent(config, "", "  ")
			gui.setConfigText(string(prettyJSON))

			gui.updateStatus("Config loaded: " + filepath.Base(gui.configPath))
			gui.appendLog("Config loaded successfully from: " + gui.configPath)
			gui.appendLog(fmt.Sprintf("  Constraints: %s", config.Constraints))
			gui.appendLog(fmt.Sprintf("  Microdata: %s", config.Microdata))
			gui.appendLog(fmt.Sprintf("  Output: %s", config.Output))
			gui.appendLog(fmt.Sprintf("  Initial Temp: %.1f", config.InitialTemp))
			gui.appendLog(fmt.Sprintf("  Max Iterations: %d", config.MaxIterations))

		}, myWindow)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		fileDialog.Show()
	})
	// Set button importance to HighImportance for blue color
	loadConfigBtn.Importance = widget.HighImportance

	// Run button - Green (Success importance)
	runBtn := widget.NewButton("Run Simulation", func() {
		if gui.configPath == "" {
			dialog.ShowInformation("Error", "Please load a config file first", myWindow)
			return
		}

		gui.updateStatus("Running simulation...")
		gui.appendLog("\nStarting simulation...")
		gui.appendLog("Config: " + filepath.Base(gui.configPath))

		go gui.runSimulation()
	})
	// Set button importance to Success for green color
	runBtn.Importance = widget.SuccessImportance

	// Clear log button - Blue
	clearBtn := widget.NewButton("Clear Log", func() {
		fyne.Do(func() {
			gui.logText.SetText("")
		})
	})
	// Set button importance to HighImportance for blue color
	clearBtn.Importance = widget.HighImportance

	buttonBar := container.NewHBox(loadConfigBtn, runBtn, clearBtn)

	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("COMPASS GUI", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			container.NewVBox(
				widget.NewLabel("Config File Contents:"),
				configScroll,
			),
			widget.NewSeparator(),
			buttonBar,
			widget.NewSeparator(),
		),
		gui.statusLabel,
		nil,
		nil,
		container.NewBorder(
			widget.NewLabel("Output Log:"),
			nil,
			nil,
			nil,
			logScroll,
		),
	)

	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(800, 700))
	myWindow.ShowAndRun()
}

func (g *GUI) runSimulation() {
	configData, err := json.MarshalIndent(g.configData, "", "  ")
	if err != nil {
		g.updateStatus("Failed to create config")
		g.appendLog("Error: " + err.Error())
		return
	}

	tempFile, err := os.CreateTemp("", "microsim_config_*.json")
	if err != nil {
		g.updateStatus("Failed to create temp config")
		g.appendLog("Error: " + err.Error())
		return
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(configData); err != nil {
		g.updateStatus("Failed to write config")
		g.appendLog("Error: " + err.Error())
		return
	}
	tempFile.Close()

	cmd := exec.Command(os.Args[0], "-f", tempFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	g.appendLog("Running simulation...")

	err = cmd.Run()
	if err != nil {
		g.updateStatus("Simulation failed")
		g.appendLog("Error: " + err.Error())
		if stderr.Len() > 0 {
			g.appendLog("Stderr: " + stderr.String())
		}
		return
	}

	var resp Resp
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		g.updateStatus("Response parsing failed")
		g.appendLog("Raw output: " + stdout.String())
		return
	}

	if resp.Status == "error" {
		g.updateStatus("Simulation error")
		g.appendLog("Error: " + resp.Message)
	} else {
		g.updateStatus("Simulation completed successfully")
		g.appendLog("Success: " + resp.Message)
	}

	for _, line := range resp.Log {
		g.appendLog("  " + line)
	}

	g.appendLog("Simulation finished")
}

/*
==============================================================
====  ENTRY POINT (main)
==============================================================
*/
func main() {
	filePath := flag.String("f", "", "path to a JSON config file")
	guiFlag := flag.Bool("g", false, "open the GUI")
	flag.Parse()

	if *guiFlag {
		runGUI()
		return
	}

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
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "No input on stdin – use -f <file>.json or pipe JSON")
			emitResponse("default", "no input provided", rMode())
			return
		}
		src = os.Stdin
	}

	cfg, err := decodeJSON(src)
	if err != nil {
		emitError("invalid JSON: " + err.Error())
		os.Exit(1)
	}

	runMicrosim(cfg)
}
