package main

import (
	"fmt"
	"github.com/minio/minio/pkg/wildcard"
	"github.com/slack-go/slack"
	"github.com/foxdalas/index-pattern-keeper/src/elastic"
	"github.com/foxdalas/index-pattern-keeper/src/kibana"
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
	var indexWithoutPattern []string
	var deletedPatterns []string

	kibana := kibana.New(os.Getenv("KIBANA_URL"))

	elastic, err := elastic.New(strings.Split(os.Getenv("ELASTICSEARCH"), ","))
	if err != nil {
		log.Fatal(err)
	}

	slackApi := slack.New(os.Getenv("SLACK_TOKEN"), slack.OptionDebug(true))


	loc, _ := time.LoadLocation("UTC")
	now := time.Now().In(loc).Format(layoutISO)

	uniqIndexes := elastic.ListIndexes()
	indexesPatterns := kibana.GetIndexesPatterns()


	for _, index := range uniqIndexes {
		var isPatternExist bool
		for _, indexPattern := range indexesPatterns {
			if wildcard.Match(indexPattern.Name, fmt.Sprintf("%s-%s", index, now)) {
				isPatternExist = true
			}
		}
		if !isPatternExist {
			fmt.Printf("Index %s without index pattern\n", index)
			indexWithoutPattern = append(indexWithoutPattern, index)
			err = kibana.CreateIndexPattern(index)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	re := regexp.MustCompile(`(.+)-\*`)
	for _, indexPattern := range indexesPatterns {
		var isIndexExist bool
		for _, index := range uniqIndexes {
			if index == re.FindStringSubmatch(indexPattern.Name)[1] {
				isIndexExist = true
			}
		}
		if !isIndexExist {
			fmt.Printf("Index pattern %s with id %s without indexes Deleting...\n", indexPattern.Name, indexPattern.ID)
			deletedPatterns = append(deletedPatterns, indexPattern.Name)
			err := kibana.DeleteIndexPattern(indexPattern.ID)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if len(indexWithoutPattern) > 0 {
		attachment := slack.Attachment{
			Pretext: fmt.Sprintf("Было создано %d index-pattern", len(indexWithoutPattern)),
			Color: "#36a64f",
			Text: strings.Join(indexWithoutPattern, "\n"),
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

	if len(deletedPatterns) > 0 {
		attachment := slack.Attachment{
			Pretext: fmt.Sprintf("Было удалено %d index-pattern", len(deletedPatterns)),
			Color: "#E01E5A",
			Text: strings.Join(deletedPatterns, "\n"),
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

