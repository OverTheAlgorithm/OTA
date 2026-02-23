package main

import (
	"context"
	"fmt"

	"ota/platform/googlenews"
	"ota/platform/googletrends"
)

func main() {
	// Google Trends
	tc := googletrends.NewCollector()
	tItems, err := tc.Collect(context.Background())
	if err != nil {
		fmt.Printf("Google Trends ERROR: %v\n", err)
	} else {
		fmt.Printf("=== Google Trends Korea: %d trending items ===\n\n", len(tItems))
		for i, item := range tItems {
			fmt.Printf("[%d] %s (traffic: %d, articles: %d)\n", i+1, item.Keyword, item.Traffic, len(item.ArticleURLs))
		}
	}

	fmt.Println("\n" + "==========================================================")

	// Google News (general feed only for quick test)
	nc := googlenews.NewCollector([]googlenews.FeedTopic{
		{Category: "general", URL: "https://news.google.com/rss?hl=ko&gl=KR&ceid=KR:ko"},
	})
	nItems, err := nc.Collect(context.Background())
	if err != nil {
		fmt.Printf("Google News ERROR: %v\n", err)
	} else {
		fmt.Printf("\n=== Google News Korea: %d topic clusters ===\n\n", len(nItems))
		for i, item := range nItems {
			fmt.Printf("[%d] %s (category: %s, articles: %d)\n", i+1, item.Keyword, item.Category, len(item.ArticleURLs))
			for j, title := range item.ArticleTitles {
				if j >= 3 {
					fmt.Printf("      ... and %d more\n", len(item.ArticleTitles)-3)
					break
				}
				fmt.Printf("      [%d] %s\n", j+1, title)
			}
		}
	}
}
