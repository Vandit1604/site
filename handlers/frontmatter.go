package handlers

import (
	"bufio"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type FrontMatter struct {
	Title string   `yaml:"title"`
	Date  string   `yaml:"date"`
	Tags  []string `yaml:"tags"`
}

// ReadFileWithFrontMatter parses the front matter and content of a markdown file
func ReadFileWithFrontMatter(slug string) (FrontMatter, string, error) {
	filePath := "content/blogs/" + slug + ".md"
	f, err := os.Open(filePath)
	if err != nil {
		return FrontMatter{}, "", err
	}
	defer f.Close()

	var frontMatter FrontMatter
	var content strings.Builder

	decoder := bufio.NewScanner(f)
	isReadingFrontMatter := false
	hasFrontMatter := false
	var frontMatterContent []string

	for decoder.Scan() {
		line := decoder.Text()

		// Start or stop reading front matter
		if line == "---" {
			if !isReadingFrontMatter {
				isReadingFrontMatter = true
				hasFrontMatter = true
			} else {
				// End of front matter
				isReadingFrontMatter = false
				continue
			}
		} else if isReadingFrontMatter {
			frontMatterContent = append(frontMatterContent, line)
		} else {
			// After front matter, add content
			content.WriteString(line + "\n")
		}
	}

	// Parse front matter if it exists
	if hasFrontMatter {
		err = yaml.Unmarshal([]byte(strings.Join(frontMatterContent, "\n")), &frontMatter)
		if err != nil {
			return FrontMatter{}, "", err
		}
	} else {
		// Handle case where no front matter exists
		return FrontMatter{}, "", nil
	}

	if err := decoder.Err(); err != nil {
		return FrontMatter{}, "", err
	}

	return frontMatter, content.String(), nil
}
