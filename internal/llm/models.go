package llm

// Tier represents a model capability level.
type Tier string

const (
	TierLite     Tier = "lite"
	TierStandard Tier = "standard"
	TierPro      Tier = "pro"
)

// ModelInfo describes a model available to the application.
type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Tier     Tier   `json:"tier"`
	FilePath string `json:"file_path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Active   bool   `json:"active"`
}
