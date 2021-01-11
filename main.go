package main

import (
	"fmt"
	"github.com/foxdalas/index-pattern-keeper/src/elastic"
	"github.com/foxdalas/index-pattern-keeper/src/kibana"
	"github.com/minio/minio/pkg/wildcard"
	"github.com/slack-go/slack"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	layoutISO = "2006.01.02"
)

func main() {
	kibana := kibana.New(os.Getenv("KIBANA_URL"))
	elastic, err := elastic.New(strings.Split(os.Getenv("ELASTICSEARCH"), ","))
	if err != nil {
		log.Fatal(err)
	}

	slackApi := slack.New(os.Getenv("SLACK_TOKEN"), slack.OptionDebug(true))
	uniqIndexes := elastic.ListIndexes()
	indexesPatterns := kibana.GetIndexesPatterns()

	indexWithoutPattern := getIndexesWithoutPattern(uniqIndexes, indexesPatterns)
	patternWithoutIndexes := getPattarnWithoutIndexes(uniqIndexes, indexesPatterns)

	kibana.FindDuplicates()

	for _, pattern := range indexWithoutPattern {
		err = kibana.CreateIndexPattern(pattern)
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, pattern := range patternWithoutIndexes {
		err = kibana.DeleteIndexPattern(pattern)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(indexWithoutPattern) > 0 {
		attachment := slack.Attachment{
			Pretext: fmt.Sprintf("Было создано %d index-pattern", len(indexWithoutPattern)),
			Color:   "#36a64f",
			Text:    strings.Join(indexWithoutPattern, "\n"),
		}
		_, _, err := slackApi.PostMessage(
			os.Getenv("CHANNEL"),
			slack.MsgOptionAttachments(attachment),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(patternWithoutIndexes) > 0 {
		attachment := slack.Attachment{
			Pretext: fmt.Sprintf("Было удалено %d index-pattern", len(patternWithoutIndexes)),
			Color:   "#E01E5A",
			Text:    strings.Join(patternWithoutIndexes, "\n"),
		}
		_, _, err := slackApi.PostMessage(
			os.Getenv("CHANNEL"),
			slack.MsgOptionAttachments(attachment),
			slack.MsgOptionAsUser(true),
		)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func getIndexesWithoutPattern(indexes []string, patterns kibana.IndexPatterns) []string {
	var indexWithoutPattern []string

	loc, _ := time.LoadLocation("UTC")
	now := time.Now().In(loc).Format(layoutISO)

	for _, index := range indexes {
		var isPatternExist bool
		for _, indexPattern := range patterns {
			if wildcard.Match(indexPattern.Name, fmt.Sprintf("%s-%s", index, now)) {
				isPatternExist = true
			}
		}
		if !isPatternExist {
			fmt.Printf("Index %s without index pattern\n", index)
			indexWithoutPattern = append(indexWithoutPattern, index)
		}
	}
	return indexWithoutPattern
}

func getPattarnWithoutIndexes(indexes []string, patterns kibana.IndexPatterns) []string {
	var patternWithoutIndexes []string

	re := regexp.MustCompile(`(.+)-\*`)
	for _, indexPattern := range patterns {
		var isIndexExist bool
		for _, index := range indexes {
			if index == re.FindStringSubmatch(indexPattern.Name)[1] {
				isIndexExist = true
			}
		}
		if !isIndexExist {
			fmt.Printf("Index pattern %s with id %s without indexes Deleting...\n", indexPattern.Name, indexPattern.ID)
			patternWithoutIndexes = append(patternWithoutIndexes, indexPattern.Name)
		}
	}
	return patternWithoutIndexes
}
