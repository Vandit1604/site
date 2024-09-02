package types

type Changelog struct {
	DateRange  string `yaml:"date_range"`
	Title      string `yaml:"title"`
	Location   string `yaml:"location"`
	Details    string `yaml:"details,omitempty"`
	DetailsURL string `yaml:"details_url,omitempty"`
	ImageURL   string `yaml:"image_url,omitempty"`
	Company    string `yaml:"company,omitempty"`
}

type PageData struct {
	Changelogs []Changelog `yaml:"changelogs"`
}
