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
func FormatMessage(items []collector.ContextItem, subscriptions []string) FormattedMessage {
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

	// Generate subject
	subject := generateSubject(selectedItems)

	// Generate text body
	textBody := generateTextBody(selectedItems)

	// Generate HTML body
	htmlBody := generateHTMLBody(selectedItems)

	return FormattedMessage{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}

func generateSubject(items []collector.ContextItem) string {
	// Count top items
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

func generateTextBody(items []collector.ContextItem) string {
	var sections []string

	// Group by category
	categoryGroups := groupByCategory(items)

	// Top category first
	if topItems, ok := categoryGroups["top"]; ok {
		sections = append(sections, "🔥 주요 화제\n"+formatItemsAsText(topItems))
	}

	// Other categories in order
	for category, catItems := range categoryGroups {
		if category != "top" {
			categoryTitle := getCategoryTitle(category)
			sections = append(sections, fmt.Sprintf("\n📌 %s\n%s", categoryTitle, formatItemsAsText(catItems)))
		}
	}

	return strings.Join(sections, "\n")
}

func generateHTMLBody(items []collector.ContextItem) string {
	var sections []string

	sections = append(sections, "<html><body style='font-family: sans-serif; line-height: 1.6;'>")

	// Group by category
	categoryGroups := groupByCategory(items)

	// Top category first
	if topItems, ok := categoryGroups["top"]; ok {
		sections = append(sections, "<h2>🔥 주요 화제</h2>")
		sections = append(sections, formatItemsAsHTML(topItems))
	}

	// Other categories
	for category, catItems := range categoryGroups {
		if category != "top" {
			categoryTitle := getCategoryTitle(category)
			sections = append(sections, fmt.Sprintf("<h2>📌 %s</h2>", categoryTitle))
			sections = append(sections, formatItemsAsHTML(catItems))
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

func formatItemsAsText(items []collector.ContextItem) string {
	var lines []string
	for i, item := range items {
		lines = append(lines, fmt.Sprintf("%d. %s: %s", i+1, item.Topic, item.Summary))
	}
	return strings.Join(lines, "\n")
}

func formatItemsAsHTML(items []collector.ContextItem) string {
	var lines []string
	lines = append(lines, "<ul>")
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("  <li><strong>%s</strong>: %s</li>", item.Topic, item.Summary))
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
