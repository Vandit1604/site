package types

type Project struct {
	Name        string `yaml:"name,omitempty"`
	GithubLink  string `yaml:"github_link,omitempty"`
	DemoLink    string `yaml:"demo_link,omitempty"`
	Description string `yaml:"description,omitempty"`
}

type Projects struct {
	Projects []Project `json:"projects"`
}
