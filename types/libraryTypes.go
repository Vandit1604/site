package types

type Link struct {
	Title       string `yaml:"title"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

type Category struct {
	Name  string `yaml:"name"`
	Links []Link `yaml:"links"`
}

type Library struct {
	Categories []Category `yaml:"categories"`
}
