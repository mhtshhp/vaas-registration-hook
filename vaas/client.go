package vaas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const (
	apiPrefixPath   = "/api/v0.1"
	apiBackendPath  = apiPrefixPath + "/backend/"
	apiDcPath       = apiPrefixPath + "/dc/"
	apiDirectorPath = apiPrefixPath + "/director/"
)

const vaasBackendIDKey = "vaas-backend-id"

const (
	contentTypeHeader = "Content-Type"
	acceptHeader      = "Accept"
	preferHeader      = "Prefer"
	applicationJSON   = "application/json"
)

// Backend represents JSON structure of backend in VaaS API.
type Backend struct {
	ID                 *int     `json:"id,omitempty"`
	Address            string   `json:"address,omitempty"`
	DirectorURL        string   `json:"director,omitempty"`
	DC                 DC       `json:"dc,omitempty"`
	Port               int      `json:"port,omitempty"`
	InheritTimeProfile bool     `json:"inherit_time_profile,omitempty"`
	Weight             *int     `json:"weight,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	ResourceURI        string   `json:"resource_uri,omitempty"`
}

// BackendList represents JSON structure of Backend list used in responses in VaaS API.
type BackendList struct {
	Meta    Meta      `json:"meta,omitempty"`
	Objects []Backend `json:"objects,omitempty"`
}

// DC represents JSON structure of DC in VaaS API.
type DC struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	ResourceURI string `json:"resource_uri,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
}

// DCList represents JSON structure of DC list used in responses in VaaS API.
type DCList struct {
	Meta    Meta `json:"meta,omitempty"`
	Objects []DC `json:"objects,omitempty"`
}

// Director represents JSON structure of Director in VaaS API.
type Director struct {
	ID          int      `json:"id,omitempty"`
	BackendURLs []string `json:"backends,omitempty"`
	Name        string   `json:"name,omitempty"`
	ResourceURI string   `json:"resource_uri,omitempty"`
}

// DirectorList represents JSON structure of Director list used in responses in VaaS API.
type DirectorList struct {
	Meta    Meta       `json:"meta,omitempty"`
	Objects []Director `json:"objects,omitempty"`
}

// Meta represents JSON structure of Meta in VaaS API.
type Meta struct {
	Limit      int     `json:"limit,omitempty"`
	Next       *string `json:"next,omitempty"`
	Offset     int     `json:"offset,omitempty"`
	Previous   *string `json:"previous,omitempty"`
	TotalCount int     `json:"total_count,omitempty"`
}

// Task represents JSON structure of a VaaS task in API.
type Task struct {
	Info        string `json:"info,omitempty"`
	ResourceURI string `json:"resource_uri,omitempty"`
}

// Client is an interface for VaaS API.
type Client interface {
	FindDirector(string) (*Director, error)
	FindDirectorID(string) (int, error)
	AddBackend(*Backend, *Director) (string, error)
	DeleteBackend(int) error
	GetDC(string) (*DC, error)
	FindBackend(director *Director, address string, port int) (*Backend, error)
	FindBackendID(director string, address string, port int) (int, error)
}

// DefaultClient is a REST client for VaaS API.
type defaultClient struct {
	httpClient *http.Client
	username   string
	apiKey     string
	host       string
}

// FindDirector finds Director by name.
func (c *defaultClient) FindDirector(name string) (*Director, error) {
	request, err := c.newRequest("GET", c.host+apiDirectorPath, nil)
	if err != nil {
		return nil, err
	}

	query := request.URL.Query()
	query.Add("name", name)
	request.URL.RawQuery = query.Encode()

	var directorList DirectorList
	if _, err = c.doRequest(request, &directorList); err != nil {
		return nil, err
	}

	for _, director := range directorList.Objects {
		if director.Name == name {
			return &director, nil
		}
	}

	return nil, fmt.Errorf("no Director with name %s found", name)
}

// FindDirectorID finds Director ID by name.
func (c *defaultClient) FindDirectorID(name string) (int, error) {
	director, err := c.FindDirector(name)
	if err != nil {
		return 0, fmt.Errorf("cannot determine director ID: %s", err)
	}
	return director.ID, nil
}

// AddBackend adds backend in VaaS director.
func (c *defaultClient) AddBackend(backend *Backend, director *Director) (string, error) {
	request, err := c.newRequest("POST", c.host+apiBackendPath, backend)
	if err != nil {
		return "", err
	}

	response, err := c.doRequest(request, backend)
	if err != nil {
		backend, newErr := c.FindBackend(director, backend.Address, backend.Port)
		if newErr != nil {
			log.Errorf("failed finding backend: %s", err)
			return "", err
		}
		return backend.ResourceURI, nil
	}

	return response.Header.Get("Location"), nil
}

// DeleteBacked removes backend with given id from VaaS director.
func (c *defaultClient) DeleteBackend(id int) error {
	request, err := c.newRequest("DELETE", fmt.Sprintf("%s%s%d/", c.host, apiBackendPath, id), nil)
	if err != nil {
		return err
	}

	request.Header.Set(preferHeader, "respond-async")
	response, err := c.do(request)
	if response != nil && response.StatusCode == http.StatusNotFound {
		log.WithField(vaasBackendIDKey, id).Warn("Tried to remove a non-existent backend")
		return nil
	}

	return err
}

// GetDC finds DC by name.
func (c *defaultClient) GetDC(name string) (*DC, error) {
	request, err := c.newRequest("GET", c.host+apiDcPath, nil)
	if err != nil {
		return nil, err
	}

	var dcList DCList
	if _, err := c.doRequest(request, &dcList); err != nil {
		return nil, err
	}

	for _, dc := range dcList.Objects {
		if dc.Symbol == name {
			return &dc, nil
		}
	}

	return nil, fmt.Errorf("no DC with name %s found", name)
}

func (c *defaultClient) FindBackendID(director string, address string, port int) (int, error) {
	directorFound, err := c.FindDirector(director)
	if err != nil {
		return 0, fmt.Errorf("cannot determine director ID: %s", err)
	}

	backend, err := c.FindBackend(directorFound, address, port)
	if err != nil {
		return 0, errors.New("backend not found")
	}
	return *backend.ID, nil
}

func (c *defaultClient) FindBackend(director *Director, address string, port int) (*Backend, error) {
	request, err := c.newRequest("GET", c.host+apiBackendPath, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create backend list request: %s", err)
	}

	query := request.URL.Query()
	query.Add("address", address)
	query.Add("director", fmt.Sprintf("%d", director.ID))
	query.Add("port", fmt.Sprintf("%d", port))
	request.URL.RawQuery = query.Encode()

	var backendList BackendList
	if _, err := c.doRequest(request, &backendList); err != nil {
		return nil, fmt.Errorf("backend list fetch failed: %s", err)
	}

	for _, backend := range backendList.Objects {
		log.Debugf("Backend found: %+v\n", backend)
		if backend.Address == address && backend.Port == port {
			return &backend, nil
		}
	}
	return nil, errors.New("backend not found")
}

func (c *defaultClient) newRequest(method, url string, body interface{}) (*http.Request, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	request.Header.Set(acceptHeader, applicationJSON)
	request.Header.Set(contentTypeHeader, applicationJSON)

	query := request.URL.Query()
	query.Add("username", c.username)
	query.Add("api_key", c.apiKey)
	request.URL.RawQuery = query.Encode()

	return request, nil
}

func (c *defaultClient) doRequest(request *http.Request, v interface{}) (*http.Response, error) {
	response, err := c.do(request)
	if err != nil {
		return response, err
	}

	rawResponse, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return response, err
	}

	if v == nil {
		return response, nil
	}
	if err := json.Unmarshal(rawResponse, v); err != nil {
		return response, err
	}

	return response, nil
}

func (c *defaultClient) do(request *http.Request) (*http.Response, error) {
	response, err := c.httpClient.Do(request)

	if err != nil {
		return response, err
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		message := ""
		rawResponse, err := ioutil.ReadAll(response.Body)
		if err != nil {
			message = fmt.Sprintf("Additional error reading raw response: %s", err.Error())
		} else {
			message = string(rawResponse)
		}
		return response, fmt.Errorf("VaaS API error at %s (HTTP %d): %s",
			request.URL, response.StatusCode, message)
	}

	return response, nil
}

// NewClient creates new REST client for VaaS API.
func NewClient(hostname string, username string, apiKey string) Client {
	return &defaultClient{
		httpClient: http.DefaultClient,
		username:   username,
		apiKey:     apiKey,
		host:       hostname,
	}
}
