package types

type Experience struct {
	DateRange        string   `yaml:"date_range"`
	Title            string   `yaml:"title"`
	Location         string   `yaml:"location"`
	Responsibilities []string `yaml:"responsibilities,omitempty"` // Field to store list of responsibilities
	DetailsURL       string   `yaml:"details_url,omitempty"`
	ImageURL         string   `yaml:"image_url,omitempty"`
	Company          string   `yaml:"company,omitempty"`
	CompanyURL       string   `yaml:"company_url,omitempty"` // Field for the company URL
}

type Experiences struct {
	Experiences []Experience `yaml:"experiences"` // Updated yaml key to "experiences"
}
