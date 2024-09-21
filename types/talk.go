package types

type Talk struct {
	Title       string `yaml:"title"`
	Date        string `yaml:"date"`
	Description string `yaml:"description"`
	VideoLink   string `yaml:"videoLink"`
	SlidesLink  string `yaml:"slidesLink"`
	ImageURL    string `yaml:"imageURL"`
}
