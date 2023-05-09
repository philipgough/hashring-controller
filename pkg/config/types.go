package config

type HashringAlgorithm string

const (
	AlgorithmHashmod         HashringAlgorithm = "hashmod"
	AlgorithmKetama          HashringAlgorithm = "ketama"
	DefaultHashringAlgorithm                   = AlgorithmKetama
)

// HashringSpec describes the hashring
type HashringSpec struct {
	// Name is the name of the hashring
	Name string `json:"hashring"`
	// Tenants is a list of tenants that should be included in the hashring
	Tenants []string `json:"tenants"`
}

// Hashring is the config for the hashring
type Hashring struct {
	HashringSpec `json:",inline"`
	Endpoints    []string `json:"endpoints"`
}

// Hashrings is a list of hashrings
type Hashrings []Hashring

func (h Hashrings) Len() int {
	return len(h)
}

func (h Hashrings) Less(i, j int) bool {
	return h[i].Name < h[j].Name
}

func (h Hashrings) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}
