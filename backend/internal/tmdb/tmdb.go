// Package tmdb provides a minimal client for The Movie Database API v3.
package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.themoviedb.org/3"
const imageBaseURL = "https://image.tmdb.org/t/p"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewFromEnv creates a Client from TMDB_API_KEY env var. Returns nil if unset.
func NewFromEnv() *Client {
	key := os.Getenv("TMDB_API_KEY")
	if key == "" {
		return nil
	}
	return New(key)
}

type SearchResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Name         string  `json:"name"` // TV shows use "name"
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	GenreIDs     []int   `json:"genre_ids"`
	MediaType    string  `json:"media_type"`
	Popularity   float64 `json:"popularity"`
}

func (r *SearchResult) DisplayTitle() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Name
}

func (r *SearchResult) Year() int {
	date := r.ReleaseDate
	if date == "" {
		date = r.FirstAirDate
	}
	if len(date) >= 4 {
		y, _ := strconv.Atoi(date[:4])
		return y
	}
	return 0
}

type genreMap map[int]string

var movieGenres = genreMap{
	28: "Action", 35: "Comedy", 80: "Crime", 99: "Documentary", 18: "Drama",
	10751: "Family", 14: "Fantasy", 36: "History", 27: "Horror", 10402: "Music",
	9648: "Mystery", 10749: "Romance", 878: "Science Fiction", 10770: "TV Movie",
	53: "Thriller", 10752: "War", 37: "Western", 16: "Animation", 12: "Adventure",
}
var tvGenres = genreMap{
	10759: "Action & Adventure", 35: "Comedy", 80: "Crime", 99: "Documentary", 18: "Drama",
	10751: "Family", 10762: "Kids", 9648: "Mystery", 10763: "News", 10764: "Reality",
	10765: "Sci-Fi & Fantasy", 10766: "Soap", 10767: "Talk", 10768: "War & Politics", 37: "Western", 16: "Animation",
}

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	params.Set("api_key", c.apiKey)
	u := baseURL + path + "?" + params.Encode()
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// SearchMovie searches TMDB for a movie by title and optional year.
func (c *Client) SearchMovie(title string, year int) (*SearchResult, error) {
	params := url.Values{"query": {title}}
	if year > 0 {
		params.Set("year", strconv.Itoa(year))
	}
	body, err := c.get("/search/movie", params)
	if err != nil {
		return nil, err
	}
	var res struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if len(res.Results) == 0 {
		return nil, fmt.Errorf("no results for %q", title)
	}
	r := &res.Results[0]
	r.MediaType = "movie"
	return r, nil
}

// SearchTV searches TMDB for a TV show by title and optional year.
func (c *Client) SearchTV(title string, year int) (*SearchResult, error) {
	params := url.Values{"query": {title}}
	if year > 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}
	body, err := c.get("/search/tv", params)
	if err != nil {
		return nil, err
	}
	var res struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if len(res.Results) == 0 {
		return nil, fmt.Errorf("no results for %q", title)
	}
	r := &res.Results[0]
	r.MediaType = "tv"
	return r, nil
}

// GenreNames resolves genre IDs to names.
func (c *Client) GenreNames(ids []int, mediaType string) []string {
	gm := movieGenres
	if mediaType == "tv" {
		gm = tvGenres
	}
	var names []string
	for _, id := range ids {
		if name, ok := gm[id]; ok {
			names = append(names, name)
		}
	}
	return names
}

// ImageURL returns the full URL for a TMDB image path.
// size: "w342" (poster), "w780" (backdrop), "original"
func ImageURL(path, size string) string {
	if path == "" {
		return ""
	}
	return imageBaseURL + "/" + size + path
}

// DownloadImage downloads a TMDB image to destPath.
func (c *Client) DownloadImage(tmdbPath, size, destPath string) error {
	if tmdbPath == "" {
		return fmt.Errorf("no image path")
	}
	imgURL := ImageURL(tmdbPath, size)
	resp, err := c.httpClient.Get(imgURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image download: %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// FetchMetadata fetches metadata for a title (movie or TV show).
// titleType should be "movie" or "series".
// Returns the best match result plus resolved genre names.
func (c *Client) FetchMetadata(title, titleType string, year int) (*SearchResult, []string, error) {
	var result *SearchResult
	var err error
	if titleType == "series" {
		result, err = c.SearchTV(title, year)
	} else {
		result, err = c.SearchMovie(title, year)
		if err != nil {
			// Fallback: try TV search
			result, err = c.SearchTV(title, year)
		}
	}
	if err != nil {
		return nil, nil, err
	}
	genres := c.GenreNames(result.GenreIDs, result.MediaType)
	return result, genres, nil
}

// Director fetches the director (movie) or creator (TV) from TMDB credits.
// Returns empty string on any error.
func (c *Client) Director(tmdbID int, mediaType string) string {
	var path string
	if mediaType == "tv" {
		path = fmt.Sprintf("/tv/%d", tmdbID)
	} else {
		path = fmt.Sprintf("/movie/%d/credits", tmdbID)
	}
	body, err := c.get(path, url.Values{})
	if err != nil {
		return ""
	}
	if mediaType == "tv" {
		var res struct {
			CreatedBy []struct {
				Name string `json:"name"`
			} `json:"created_by"`
		}
		if json.Unmarshal(body, &res) == nil && len(res.CreatedBy) > 0 {
			names := make([]string, len(res.CreatedBy))
			for i, cb := range res.CreatedBy {
				names[i] = cb.Name
			}
			return strings.Join(names, ", ")
		}
	} else {
		var res struct {
			Crew []struct {
				Job  string `json:"job"`
				Name string `json:"name"`
			} `json:"crew"`
		}
		if json.Unmarshal(body, &res) == nil {
			for _, c := range res.Crew {
				if c.Job == "Director" {
					return c.Name
				}
			}
		}
	}
	return ""
}
