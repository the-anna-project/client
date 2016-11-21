package collection

import (
	servicespec "github.com/the-anna-project/spec/service"
)

// New creates a new endpoint collection.
func New() servicespec.EndpointCollection {
	return &collection{}
}

type collection struct {
	// Dependencies.

	textService servicespec.EndpointService
}

func (c *collection) Boot() {
	go c.Text().Boot()
}

func (c *collection) SetTextService(textService servicespec.EndpointService) {
	c.textService = textService
}

func (c *collection) Text() servicespec.EndpointService {
	return c.textService
}
