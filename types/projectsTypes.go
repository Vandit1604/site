package types

type Project struct {
	Name        string `yaml:"name,omitempty"`
	GithubLink  string `yaml:"github_link,omitempty"`
	DemoLink    string `yaml:"demo_link,omitempty"`
	Description string `yaml:"description,omitempty"`
	CollabName  string `yaml:"collab_name,omitempty"` // optional co-builder credit
	CollabLink  string `yaml:"collab_link,omitempty"`
	// Art names an inline diagram template in projectart.html ("dockerium"
	// renders {{ template "art:dockerium" }}). Empty means the card renders
	// text-only, which is the correct look for a project with nothing
	// meaningful to draw.
	Art string `yaml:"art,omitempty"`
	// Tags are the short pills along the bottom of the card, matching the
	// indie products above them.
	Tags []string `yaml:"tags,omitempty"`
}

type Projects struct {
	Projects []Project `json:"projects"`
}
