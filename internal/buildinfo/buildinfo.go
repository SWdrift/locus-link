package buildinfo

const PreviousVersion = "0.1.0"

var (
	Version  = "dev"
	Commit   = "unknown"
	Artifact = "development"
)

type Info struct {
	Version            string `json:"version"`
	PreviousVersion    string `json:"previous_version"`
	StateSchemaVersion int    `json:"state_schema_version"`
	Commit             string `json:"commit"`
	Artifact           string `json:"artifact"`
}
