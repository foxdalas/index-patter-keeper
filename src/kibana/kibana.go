package kibana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type Kibana struct {
	Url     string
	Context *context.Context
	Logger  *log.Entry
	client  *http.Client
}

type NewIndexPattern struct {
	Attributes struct {
		Title         string `json:"title"`
		TimeFieldName string `json:"timeFieldName"`
	} `json:"attributes"`
}

type KibanaIndexPatterns struct {
	Page         int `json:"page"`
	PerPage      int `json:"per_page"`
	Total        int `json:"total"`
	SavedObjects []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Title         string `json:"title"`
			TimeFieldName string `json:"timeFieldName"`
			Fields        string `json:"fields"`
		} `json:"attributes"`
		References       []interface{} `json:"references"`
		MigrationVersion struct {
			IndexPattern string `json:"index-pattern"`
		} `json:"migrationVersion"`
		UpdatedAt  time.Time `json:"updated_at"`
		Version    string    `json:"version"`
		Namespaces []string  `json:"namespaces"`
		Score      int       `json:"score"`
	} `json:"saved_objects"`
}

type IndexPattern struct {
	ID   string
	Name string
}

type IndexPatterns []IndexPattern

const magic_header = "osd-xsrf"
const index_pattern_title = "*:%s-*"

func New(url string, timeout time.Duration, logger *log.Entry) (*Kibana, error) {
	client := &http.Client{
		Timeout: timeout,
	}
	return &Kibana{
		Url:    url,
		Logger: logger,
		client: client,
	}, nil
}

func (k *Kibana) DeleteDuplicates() error {
	duplicates := make(map[string][]IndexPattern)

	patterns, err := k.GetIndexesPatterns()
	if err != nil {
		k.Logger.Error("Kibana: can't get index patterns: %v", err)
		return err
	}

	for _, pattern := range patterns {
		duplicates[pattern.Name] = append(duplicates[pattern.Name], pattern)
	}

	for _, ids := range duplicates {
		if len(ids) > 1 {
			fmt.Println(k)
			err := k.DeleteIndexPattern(&ids[1])
			if err != nil {
				k.Logger.Error("Kibana: can't delete index pattern: %v", err)
				return err
			}
			k.Logger.Infof("Kibana: deleted duplicate index pattern %s", ids[1].ID)
		}
	}
	return nil
}

func (k *Kibana) CreateIndexPattern(index string) error {
	data := &NewIndexPattern{}
	data.Attributes.TimeFieldName = "@timestamp"
	data.Attributes.Title = fmt.Sprintf(index_pattern_title, index)
	b, err := json.Marshal(data)
	if err != nil {
		k.Logger.Errorf("Kibana: JSON marshal error %v", err)
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/saved_objects/index-pattern", k.Url), bytes.NewBuffer(b))
	if err != nil {
		k.Logger.Error(err)
		return err
	}
	req.Header.Add(magic_header, "true")
	resp, err := k.client.Do(req)
	if err != nil {
		k.Logger.Errorf("Kibana: can't create index pattern %s %v", index, err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			k.Logger.Errorf("Kibana: request body close error %v", err)
		}
	}(resp.Body)
	k.Logger.Infof("Kibana: creating pattern for %s success, response status: %d", index, resp.StatusCode)
	return err
}

func (k *Kibana) DeleteIndexPattern(pattern *IndexPattern) error {
	id := pattern.ID

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/saved_objects/index-pattern/%s", k.Url, id), nil)
	if err != nil {
		k.Logger.Errorf("Kibana: can't create pattern delete request: %v", err)
		return err
	}
	req.Header.Add(magic_header, "true")
	resp, err := k.client.Do(req)
	if err != nil {
		k.Logger.Errorf("Kibana: can't delete index patterns: %v", err)
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			k.Logger.Errorf("Kibana: request body close error %v", err)
		}
	}(resp.Body)
	k.Logger.Infof("Kibana: Deleted pattern, %s response status: %d", pattern.Name, resp.StatusCode)
	return err
}

func (k *Kibana) GetIndexesPatterns() (IndexPatterns, error) {
	var data IndexPatterns
	res := KibanaIndexPatterns{}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/saved_objects/_find?per_page=1000&type=index-pattern&search_fields=title&search=*", k.Url), nil)
	resp, err := k.client.Do(req)
	if err != nil {
		k.Logger.Errorf("Kibana: can't get index patterns: %v", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			k.Logger.Errorf("Kibana: request body close error: %v", err)
		}
	}(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &res)
	if err != nil {
		k.Logger.Errorf("Kibana: JSON unmarshal error: %v", err)
		return nil, err
	}
	for _, pattern := range res.SavedObjects {
		pattern := IndexPattern{
			ID:   pattern.ID,
			Name: pattern.Attributes.Title,
		}
		data = append(data, pattern)
	}
	return data, nil
}
