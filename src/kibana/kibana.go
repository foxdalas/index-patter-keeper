package kibana

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Kibana struct {
	Url string
}

type CreateIndexPattern struct {
	Attributes struct {
		Title         string `json:"title"`
		TimeFieldName string `json:"timeFieldName"`
	} `json:"attributes"`
}

type KibanaIncexesPatterns struct {
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


func New(url string) *Kibana {
	return &Kibana{
		Url: url,
	}
}

func (k *Kibana) CreateIndexPattern(index string) error {
	data := &CreateIndexPattern{}
	data.Attributes.TimeFieldName = "@timestamp"
	data.Attributes.Title = fmt.Sprintf("%s-*", index)
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/saved_objects/index-pattern/%s-*?overwrite=true", k.Url, index), bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Add("kbn-xsrf", "true")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	return err
}

func (k *Kibana) DeleteIndexPattern(id string) error {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/saved_objects/index-pattern/%s", k.Url, id), nil)
	if err != nil {
		return err
	}
	req.Header.Add("kbn-xsrf", "true")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	return err
}

func (k *Kibana) GetIndexesPatterns() IndexPatterns {
	var data IndexPatterns
	res := KibanaIncexesPatterns{}
	resp, err := http.Get(fmt.Sprintf("%s/api/saved_objects/_find?per_page=1000&type=index-pattern&search_fields=title&search=*", k.Url))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &res)
	if err != nil {
		log.Fatal(err)
	}
	for _, pattern := range res.SavedObjects {
		pattern := IndexPattern{
			ID:   pattern.ID,
			Name: pattern.Attributes.Title,
		}
		data = append(data, pattern)
	}
	return data
}
