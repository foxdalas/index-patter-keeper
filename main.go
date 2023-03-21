package main

import (
	"context"
	"fmt"
	"github.com/foxdalas/index-pattern-keeper/src/elastic"
	"github.com/foxdalas/index-pattern-keeper/src/kibana"
	"github.com/foxdalas/index-pattern-keeper/src/tools"
	"github.com/minio/pkg/wildcard"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	log "github.com/sirupsen/logrus"
)

const (
	layoutISO = "2006.01.02"
	app       = "index-pattern-keeper"
	// Regex for split index name and date
	// Date can be with format YYYY-MM-DD
	// And also date can be with formant YYY.MM.DD
	dateRegex = "-[0-9]{4}-[0-9]{2}-[0-9]{2}|-[0-9]{4}\\.[0-9]{2}\\.[0-9]{2}"
)

var (
	// Internal indexes which used by kibana and opensearch
	// And we don't want to create index pattern for them
	internalIndexes = []string{
		".*",
		"security-auditlog-*",
		"*_elastalert_*",
		"opensearch_*",
		"opendistro-*",
	}
)

type options struct {
	LogType           string        `env:"LOG_TYPE" envDefault:"text"`
	LogLevel          string        `env:"LOG_LEVEL" envDefault:"info"`
	OpensearchServers []string      `env:"OPENSEARCH_SERVERS" envDefault:"http://localhost:9200" envSeparator:","`
	KibanaServer      string        `env:"KIBANA_SERVER" envDefault:"http://localhost:5601"`
	IndexMatchPattern string        `env:"INDEX_PATTERN" envDefault:"*"`
	ExcludeIndexes    []string      `env:"EXCLUDE_INDEXES" envSeparator:","`
	ConnectTimeout    time.Duration `env:"CONNECT_TIMEOUT" envDefault:"30s"`
}

func parseOptions() (*options, error) {
	options := options{}
	if err := env.Parse(&options); err != nil {
		return nil, err
	}
	if len(options.OpensearchServers) == 0 {
		fmt.Print("Cannot found elastic search services - set the OPENSEARCH_SERVERS environment variable\n")
		os.Exit(1)
	}
	if len(options.KibanaServer) == 0 {
		fmt.Print("Cannot found kibana services - set the KIBANA_SERVERS environment variable\n")
		os.Exit(1)
	}
	return &options, nil
}

func initLog(o *options) *log.Entry {
	switch strings.ToLower(o.LogType) {
	case "text":
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
	case "json":
		log.SetFormatter(&log.JSONFormatter{
			TimestampFormat: "02.01.2006 15:04:05",
			FieldMap: log.FieldMap{
				log.FieldKeyMsg:  "message",
				log.FieldKeyTime: "@timestamp",
			},
		})
	default:
		log.SetFormatter(&log.TextFormatter{
			ForceColors: true,
		})
	}

	switch strings.ToLower(o.LogLevel) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	return log.WithFields(log.Fields{
		"app": app,
	})
}

func main() {

	options, err := parseOptions()
	if err != nil {
		panic(err)
	}

	logger := initLog(options)
	logger.Info("Start app")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// Signal processing
	go func() {
		sig := <-sigChan
		logger.Infof("Exit by signal \"%s\"", sig.String())
		os.Exit(0)
	}()

	// init kibana
	kib, err := kibana.New(options.KibanaServer, options.ConnectTimeout, logger)
	if err != nil {
		logger.Fatalf("Error creating kibana structure for host %s: %v", options.KibanaServer, err)
	}

	// fill indexes and patters
	excludeIndexes := tools.UniqueNonEmptyElementsOf(append(internalIndexes, options.ExcludeIndexes...))
	indexes := queryAllElastics(options.OpensearchServers, options.ConnectTimeout, options.IndexMatchPattern, excludeIndexes, logger)
	logger.Debugf("Found %d indexes", len(indexes))
	patterns, err := kib.GetIndexesPatterns()
	if err != nil {
		logger.Fatalf("Error getting index patterns: %v", err)
	}
	logger.Debugf("Found %d patterns", len(patterns))

	indexesWithoutPattern := findIndexesWithoutPattern(indexes, patterns, logger)
	patternsWithoutIndex := findPatternWithoutIndexes(indexes, patterns, logger)

	// Delete patterns which have no index
	if len(patternsWithoutIndex) > 0 {
		logger.Infof("Found %d patterns without index", len(patternsWithoutIndex))
		// TODO: Delete index patterns
		for _, pattern := range patternsWithoutIndex {
			err = kib.DeleteIndexPattern(&pattern)
			if err != nil {
				logger.Fatalf("Error deleting index pattern %s: %v", pattern.Name, err)
			}
		}
	}

	// Creating patterns for index which not exist
	if len(indexesWithoutPattern) > 0 {
		logger.Infof("Found %d indexes without pattern", len(indexesWithoutPattern))
		for _, index := range indexesWithoutPattern {
			err = kib.CreateIndexPattern(index)
			if err != nil {
				logger.Fatalf("Error creating index pattern %s: %v", index, err)
			}
		}
	} else {
		logger.Info("All indexes have pattern, do nothing")
	}

	//TODO: check duplicates in index patterns
	err = kib.DeleteDuplicates()
	if err != nil {
		logger.Fatalf("Error finding duplicates: %v", err)
	}
	logger.Infof("All done, exit")
	os.Exit(0)
}

func queryAllElastics(clusters []string, timeout time.Duration, pattern string, excludeIndexes []string, logger *log.Entry) []string {

	var globalIndexes []string
	var result []string
	var include bool

	re := regexp.MustCompile(dateRegex)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	for _, cluster := range clusters {
		client, err := elastic.New(ctx, cluster, logger)
		if err != nil {
			logger.Fatalf("Error connecting to elastic host %s: %v", cluster, err)
			continue
		}

		res, err := client.CatIndexes(pattern)
		if err != nil {
			logger.Fatalf("Error getting indexes: %v", err)
			continue
		}
		globalIndexes = append(globalIndexes, res...)
	}

	for _, index := range globalIndexes {
		include = true
		resultString := re.Split(index, -1)
		for _, pattern := range excludeIndexes {
			if wildcard.Match(pattern, resultString[0]) {
				include = false
				continue
			}
		}
		if include {
			result = append(result, resultString[0])
		}
	}

	defer cancel()

	return tools.UniqueNonEmptyElementsOf(result)
}

func findIndexesWithoutPattern(indexes []string, patterns kibana.IndexPatterns, logger *log.Entry) []string {
	var indexWithoutPattern []string

	loc, _ := time.LoadLocation("UTC")
	now := time.Now().In(loc).Format(layoutISO)

	for _, index := range indexes {
		var isPatternExist bool
		for _, indexPattern := range patterns {
			_, after, _ := strings.Cut(indexPattern.Name, ":")
			if wildcard.Match(after, fmt.Sprintf("%s-%s", index, now)) { // TODO: move out wildcard lib
				isPatternExist = true
			}
		}
		if !isPatternExist {
			indexWithoutPattern = append(indexWithoutPattern, index)
		}
	}
	sort.Strings(indexWithoutPattern)
	logger.Infof("Found %d indexes without index pattern", len(indexWithoutPattern))
	return indexWithoutPattern
}

func findPatternWithoutIndexes(indexes []string, patterns kibana.IndexPatterns, logger *log.Entry) []kibana.IndexPattern {
	var patternsWithoutIndexes []kibana.IndexPattern

	re := regexp.MustCompile(`\*:(.+)-\*`)
	for _, indexPattern := range patterns {
		var isIndexExist bool
		for _, index := range indexes {
			for _, ptr := range re.FindStringSubmatch(indexPattern.Name) {
				if index == ptr {
					isIndexExist = true
				}
			}
		}
		if !isIndexExist {
			patternsWithoutIndexes = append(patternsWithoutIndexes, indexPattern)
		}
	}
	logger.Infof("Found %d patterns without indexes", len(patternsWithoutIndexes))
	return patternsWithoutIndexes
}
