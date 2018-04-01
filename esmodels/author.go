package esmodels

type Author struct {
	Name         string   `json:"name" esType:"keyword"`
	PrimaryURL   string   `json:"primary_url" esType:"keyword"`
	Created      string   `json:"created" esType:"date"`
	LastUpdated  string   `json:"last_updated" esType:"date"`
	Repositories []string `json:"name" esType:"keyword"`
}
