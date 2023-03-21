package elastic

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"

	"github.com/foxdalas/index-pattern-keeper/src/tools"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"strings"
)

type OpenSearch struct {
	Ctx    context.Context
	Client *opensearch.Client
	Logger *log.Entry
}

func New(ctx context.Context, hostname string, log *log.Entry) (*OpenSearch, error) {
	var addresses []string

	addresses = append(addresses, hostname)

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses:         addresses,
		EnableDebugLogger: true,
	})
	if err != nil {
		return nil, err
	}

	return &OpenSearch{
		Client: client,
		Ctx:    ctx,
		Logger: log,
	}, nil
}

func (o *OpenSearch) CatIndexes(indexPattern string) ([]string, error) {
	var data []string

	getIndexes := opensearchapi.CatIndicesRequest{
		Index: []string{indexPattern}, // get index names matching pattern
		H:     []string{"index"},      // fetch only index names
		// Format: "json",
	}

	res, err := getIndexes.Do(o.Ctx, o.Client)
	if err != nil {
		o.Logger.Errorf("Elasitc: problem with connection to elastic: %v", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			o.Logger.Errorf("Elasitc: problem with getting indexes: %v", err)
		}
	}(res.Body)
	if res.IsError() {
		return nil, fmt.Errorf("error getting indexes: %s", res.String())
	}
	if err != nil {
		o.Logger.Fatalf("Elastic: problem with getting indexes: %v", err)
		return nil, err
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		o.Logger.Errorf("Elastic: error reading response body: %v", err)
		return nil, err
	}

	data = strings.Split(string(b), "\n")
	return tools.UniqueNonEmptyElementsOf(data), nil
}
