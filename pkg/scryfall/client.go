package scryfall

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL   = "https://api.scryfall.com"
	UserAgent = "MTGDiscordBot/1.0"
	RateLimit = 100 * time.Millisecond // 10 requests per second as recommended
)

type Client struct {
	httpClient  *http.Client
	rateLimiter *time.Ticker
}

type Card struct {
	Object       string            `json:"object"`
	ID           string            `json:"id"`
	OracleID     string            `json:"oracle_id"`
	Name         string            `json:"name"`
	Lang         string            `json:"lang"`
	ReleasedAt   string            `json:"released_at"`
	URI          string            `json:"uri"`
	ScryfallURI  string            `json:"scryfall_uri"`
	Layout       string            `json:"layout"`
	ImageUris    map[string]string `json:"image_uris,omitempty"`
	CardFaces    []CardFace        `json:"card_faces,omitempty"`
	ManaCost     string            `json:"mana_cost,omitempty"`
	CMC          float64           `json:"cmc"`
	TypeLine     string            `json:"type_line"`
	OracleText   string            `json:"oracle_text,omitempty"`
	Colors       []string          `json:"colors,omitempty"`
	SetName      string            `json:"set_name"`
	SetCode      string            `json:"set"`
	Rarity       string            `json:"rarity"`
	Artist       string            `json:"artist,omitempty"`
	Prices       Prices            `json:"prices"`
	ImageStatus  string            `json:"image_status"`
	HighresImage bool              `json:"highres_image"`
}

type CardFace struct {
	Object     string            `json:"object"`
	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost"`
	TypeLine   string            `json:"type_line"`
	OracleText string            `json:"oracle_text,omitempty"`
	Colors     []string          `json:"colors,omitempty"`
	Artist     string            `json:"artist,omitempty"`
	ImageUris  map[string]string `json:"image_uris,omitempty"`
}

type Prices struct {
	USD     *string `json:"usd"`
	USDFoil *string `json:"usd_foil"`
	EUR     *string `json:"eur"`
	EURFoil *string `json:"eur_foil"`
	Tix     *string `json:"tix"`
}

type SearchResult struct {
	Object     string `json:"object"`
	TotalCards int    `json:"total_cards"`
	HasMore    bool   `json:"has_more"`
	NextPage   string `json:"next_page,omitempty"`
	Data       []Card `json:"data"`
}

type ErrorResponse struct {
	Object   string   `json:"object"`
	Code     string   `json:"code"`
	Status   int      `json:"status"`
	Details  string   `json:"details"`
	Type     string   `json:"type,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("scryfall api error: %s (status: %d)", e.Details, e.Status)
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		rateLimiter: time.NewTicker(RateLimit),
	}
}

func (c *Client) request(endpoint string) (*http.Response, error) {
	// Rate limiting
	<-c.rateLimiter.C

	req, err := http.NewRequest("GET", BaseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				// Log error but don't fail the function
				fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
			}
		}()
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("http error %d", resp.StatusCode)
		}
		return nil, errResp
	}

	return resp, nil
}

// GetCardByName searches for a card by name using fuzzy matching
func (c *Client) GetCardByName(name string) (*Card, error) {
	if name == "" {
		return nil, fmt.Errorf("card name cannot be empty")
	}

	endpoint := fmt.Sprintf("/cards/named?fuzzy=%s", url.QueryEscape(name))

	resp, err := c.request(endpoint)
	if err != nil {
		return nil, fmt.Errorf("requesting card by name: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	var card Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decoding card response: %w", err)
	}

	return &card, nil
}

// GetCardByExactName searches for a card by exact name match
func (c *Client) GetCardByExactName(name string) (*Card, error) {
	if name == "" {
		return nil, fmt.Errorf("card name cannot be empty")
	}

	endpoint := fmt.Sprintf("/cards/named?exact=%s", url.QueryEscape(name))

	resp, err := c.request(endpoint)
	if err != nil {
		return nil, fmt.Errorf("requesting card by exact name: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	var card Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decoding card response: %w", err)
	}

	return &card, nil
}

// GetRandomCard returns a random Magic card
func (c *Client) GetRandomCard() (*Card, error) {
	endpoint := "/cards/random"

	resp, err := c.request(endpoint)
	if err != nil {
		return nil, fmt.Errorf("requesting random card: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	var card Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("decoding random card response: %w", err)
	}

	return &card, nil
}

// SearchCards performs a full-text search for cards
func (c *Client) SearchCards(query string) (*SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	endpoint := fmt.Sprintf("/cards/search?q=%s", url.QueryEscape(query))

	resp, err := c.request(endpoint)
	if err != nil {
		return nil, fmt.Errorf("searching cards: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log error but don't fail the function
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	return &result, nil
}

// Close stops the rate limiter ticker
func (c *Client) Close() {
	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}
}

// GetBestImageURL returns the highest quality image URL available for a card
func (c *Card) GetBestImageURL() string {
	var imageUris map[string]string

	// For double-faced cards, prefer the first face
	if len(c.CardFaces) > 0 && c.CardFaces[0].ImageUris != nil {
		imageUris = c.CardFaces[0].ImageUris
	} else if c.ImageUris != nil {
		imageUris = c.ImageUris
	} else {
		return ""
	}

	// Prefer highest quality images in order
	imagePreference := []string{"png", "large", "normal", "small"}

	for _, format := range imagePreference {
		if url, exists := imageUris[format]; exists {
			return url
		}
	}

	// Return any available image if none of the preferred formats exist
	for _, url := range imageUris {
		return url
	}

	return ""
}

// GetDisplayName returns the appropriate display name for the card
func (c *Card) GetDisplayName() string {
	if c.Name != "" {
		return c.Name
	}

	// For multi-faced cards without a combined name
	if len(c.CardFaces) > 0 {
		names := make([]string, len(c.CardFaces))
		for i, face := range c.CardFaces {
			names[i] = face.Name
		}
		return strings.Join(names, " // ")
	}

	return "Unknown Card"
}

// IsValidCard checks if the card has valid data for display
func (c *Card) IsValidCard() bool {
	return c.Object == "card" && (c.Name != "" || len(c.CardFaces) > 0)
}

// HasImage checks if the card has at least one image available
func (c *Card) HasImage() bool {
	return c.GetBestImageURL() != ""
}
