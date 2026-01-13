


# -------------------------------------------------
# R script: test.R
# -------------------------------------------------
library(jsonlite)

# 1️  Build a nested list that mirrors the Go structs
payload <- list(
  constraints = "data/BlockWorld/artifical_cencus.csv",
  microdata   = "data/BlockWorld/artifical_survay.csv",
  groups      = "data/BlockWorld/artificial_groups.csv",
  output      = "results/artificial_synthetic_population.csv",
  validate    = "results/artificial_synthPopSurvey.csv",
  initialTemp      = 1000.0,
  minTemp          = 0.001,
  coolingRate      = 0.997,
  reheatFactor     = 0.3,
  fitnessThreshold = 0.01,
  minImprovement   = 0.001,
  maxIterations    = 500000,
  windowSize       = 5000,
  change           = 100000,
  distance         = "NORM_EUCLIDEAN",
  useRandomSeed    = "no",
  randomSeed       = 42
)

# 2️ Convert the list to compact JSON (one line, no pretty‑printing)
json_in <- toJSON(payload, auto_unbox = TRUE, pretty = FALSE)

# 3️  Call the Go binary, feeding the JSON via stdin
out_json <- system2(
  "./COMPASS",          # path to the compiled binary
  input = json_in,
  stdout = TRUE,          # capture stdout (the result JSON)
  stderr = FALSE          # capture stderr 
)

# 4️  Turn the returned JSON back into an R object
result <- fromJSON(paste(out_json, collapse = "\n"))
print(result)   # should show list(status = "ok")