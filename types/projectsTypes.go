package types

type Project struct {
	Name        string `yaml:"name,omitempty"`
	GithubLink  string `yaml:"github_link,omitempty"`
	DemoLink    string `yaml:"demo_link,omitempty"`
	Description string `yaml:"description,omitempty"`
	CollabName  string `yaml:"collab_name,omitempty"` // optional co-builder credit
	CollabLink  string `yaml:"collab_link,omitempty"`
}

type Projects struct {
	Projects []Project `json:"projects"`
}
