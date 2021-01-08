package elastic

import (
	"context"
	"errors"
	"fmt"
	"github.com/olivere/elastic/v7"
	"github.com/opentracing/opentracing-go/log"
	"net/http"
	"regexp"
	"syscall"
	"time"
)

const (
	layoutISO = "2006.01.02"
)

type elasticSearch struct {
	Ctx    context.Context
	Client *elastic.Client
}

type EsRetrier struct {
	backoff elastic.Backoff
}

func New(elasticHost []string) (*elasticSearch, error) {
	client, err := elastic.NewClient(
		elastic.SetURL(elasticHost...),
		elastic.SetSniff(false),
		elastic.SetRetrier(NewEsRetrier()),
		elastic.SetHealthcheck(true),
		elastic.SetHealthcheckTimeout(time.Second*300),
	)
	if err != nil {
		return nil, err
	}

	ctx, _ := context.WithTimeout(context.Background(), 300*time.Second)

	return &elasticSearch{
		Client: client,
		Ctx:    ctx,
	}, nil
}

func ( e *elasticSearch) ListIndexes() []string {
	var data []string
	indexName := fmt.Sprintf("*-2*")
	validIndex := regexp.MustCompile(`^.+-2.+`)
	re := regexp.MustCompile(`(.+)-\d{4}.+`)
	res, err := e.Client.IndexGetSettings().Index(indexName).Do(e.Ctx)
	if err != nil {
		log.Error(err)
	}
	var names []string
	for name := range res {
		names = append(names, name)
	}

	for _, name := range names {
		if validIndex.MatchString(name) {
			if len(re.FindStringSubmatch(name)[1]) > 0 {
				data = append(data, re.FindStringSubmatch(name)[1])
			}
		}
	}
	return uniqueNonEmptyElementsOf(data)
}

func NewEsRetrier() *EsRetrier {
	return &EsRetrier{
		backoff: elastic.NewExponentialBackoff(10*time.Millisecond, 8*time.Second),
	}
}

func (r *EsRetrier) Retry(ctx context.Context, retry int, req *http.Request, resp *http.Response, err error) (time.Duration, bool, error) {
	if err == syscall.ECONNREFUSED {
		return 0, false, errors.New("Elasticsearch or network down")
	}

	if retry >= 5 {
		return 0, false, nil
	}

	wait, stop := r.backoff.Next(retry)
	return wait, stop, nil
}

func uniqueNonEmptyElementsOf(s []string) []string {
	unique := make(map[string]bool, len(s))
	us := make([]string, len(unique))
	for _, elem := range s {
		if len(elem) != 0 {
			if !unique[elem] {
				us = append(us, elem)
				unique[elem] = true
			}
		}
	}

	return us

}
