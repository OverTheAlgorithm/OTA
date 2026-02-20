package delivery

import (
	"fmt"
	"strings"

	"ota/domain/collector"
)

// FormatMessage creates a personalized message from context items.
// This is a pure function - no side effects, completely testable.
//
// Rules:
// - Always include items from "top" category (universal topics)
// - Append items from subscribed categories
// - One sentence per topic (max 2 if necessary)
// - Output in Korean
// - frontendURL is used to generate "자세히 말해주세요" links per item.
//   Pass empty string to omit links (e.g. in tests or when URL is unavailable).
func FormatMessage(items []collector.ContextItem, subscriptions []string, frontendURL string) FormattedMessage {
	if len(items) == 0 {
		return FormattedMessage{
			Subject:  "오늘의 맥락",
			TextBody: "오늘은 수집된 맥락이 없습니다.",
			HTMLBody: "<p>오늘은 수집된 맥락이 없습니다.</p>",
		}
	}

	// Build subscription set for fast lookup
	subSet := make(map[string]bool)
	for _, sub := range subscriptions {
		subSet[sub] = true
	}

	// Filter items: always include "top", plus subscribed categories
	var selectedItems []collector.ContextItem
	for _, item := range items {
		if item.Category == "top" || subSet[item.Category] {
			selectedItems = append(selectedItems, item)
		}
	}

	if len(selectedItems) == 0 {
		return FormattedMessage{
			Subject:  "오늘의 맥락",
			TextBody: "구독하신 주제에 대한 맥락이 없습니다.",
			HTMLBody: "<p>구독하신 주제에 대한 맥락이 없습니다.</p>",
		}
	}

	subject := generateSubject(selectedItems)
	textBody := generateTextBody(selectedItems, frontendURL)
	htmlBody := generateHTMLBody(selectedItems, frontendURL)

	return FormattedMessage{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}

func generateSubject(items []collector.ContextItem) string {
	topCount := 0
	for _, item := range items {
		if item.Category == "top" {
			topCount++
		}
	}

	if topCount == len(items) {
		return fmt.Sprintf("오늘의 맥락 %d가지", topCount)
	}

	return fmt.Sprintf("오늘의 맥락 %d가지 (구독 주제 포함)", len(items))
}

func generateTextBody(items []collector.ContextItem, frontendURL string) string {
	var sections []string

	categoryGroups := groupByCategory(items)

	if topItems, ok := categoryGroups["top"]; ok {
		sections = append(sections, "🔥 주요 화제\n"+formatItemsAsText(topItems, frontendURL))
	}

	for category, catItems := range categoryGroups {
		if category != "top" {
			categoryTitle := getCategoryTitle(category)
			sections = append(sections, fmt.Sprintf("\n📌 %s\n%s", categoryTitle, formatItemsAsText(catItems, frontendURL)))
		}
	}

	return strings.Join(sections, "\n")
}

func generateHTMLBody(items []collector.ContextItem, frontendURL string) string {
	var sections []string

	sections = append(sections, "<html><body style='font-family: sans-serif; line-height: 1.6;'>")

	categoryGroups := groupByCategory(items)

	if topItems, ok := categoryGroups["top"]; ok {
		sections = append(sections, "<h2>🔥 주요 화제</h2>")
		sections = append(sections, formatItemsAsHTML(topItems, frontendURL))
	}

	for category, catItems := range categoryGroups {
		if category != "top" {
			categoryTitle := getCategoryTitle(category)
			sections = append(sections, fmt.Sprintf("<h2>📌 %s</h2>", categoryTitle))
			sections = append(sections, formatItemsAsHTML(catItems, frontendURL))
		}
	}

	sections = append(sections, "</body></html>")

	return strings.Join(sections, "\n")
}

func groupByCategory(items []collector.ContextItem) map[string][]collector.ContextItem {
	groups := make(map[string][]collector.ContextItem)
	for _, item := range items {
		groups[item.Category] = append(groups[item.Category], item)
	}
	return groups
}

func formatItemsAsText(items []collector.ContextItem, frontendURL string) string {
	var lines []string
	for i, item := range items {
		line := fmt.Sprintf("%d. %s: %s", i+1, item.Topic, item.Summary)
		if frontendURL != "" && item.Detail != "" {
			line += fmt.Sprintf("\n   👉 자세히 보기: %s/topic/%s", frontendURL, item.ID)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func formatItemsAsHTML(items []collector.ContextItem, frontendURL string) string {
	var lines []string
	lines = append(lines, "<ul>")
	for _, item := range items {
		entry := fmt.Sprintf("  <li><strong>%s</strong>: %s", item.Topic, item.Summary)
		if frontendURL != "" && item.Detail != "" {
			entry += fmt.Sprintf(
				`<br><a href="%s/topic/%s" style="font-size:0.85em;color:#5ba4d9;text-decoration:none;">자세히 말해주세요 →</a>`,
				frontendURL, item.ID,
			)
		}
		entry += "</li>"
		lines = append(lines, entry)
	}
	lines = append(lines, "</ul>")
	return strings.Join(lines, "\n")
}

func getCategoryTitle(category string) string {
	titles := map[string]string{
		"entertainment": "연예",
		"economy":       "경제",
		"sports":        "스포츠",
		"politics":      "정치",
		"technology":    "기술",
		"society":       "사회",
	}

	if title, ok := titles[category]; ok {
		return title
	}
	return category
}
