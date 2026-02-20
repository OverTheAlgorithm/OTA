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
	categoryGroups := groupByCategory(items)

	var body strings.Builder

	if topItems, ok := categoryGroups["top"]; ok {
		body.WriteString(renderEmailSection("전체 맥락", "#e84d3d", topItems, frontendURL))
	}

	for category, catItems := range categoryGroups {
		if category != "top" {
			body.WriteString(renderEmailSection(getCategoryTitle(category), "#5ba4d9", catItems, frontendURL))
		}
	}

	return wrapEmailTemplate(body.String())
}

func wrapEmailTemplate(content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#0f0a19;font-family:'Apple SD Gothic Neo','Malgun Gothic','Noto Sans KR',sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#0f0a19;">
  <tr><td align="center" style="padding:32px 16px 48px;">
    <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;">

      <!-- Header -->
      <tr><td style="padding-bottom:28px;">
        <p style="margin:0;font-size:22px;font-weight:700;color:#f5f0ff;letter-spacing:-0.03em;">OTA</p>
        <p style="margin:4px 0 0;font-size:13px;color:#9b8bb4;letter-spacing:0.01em;">오늘의 맥락 브리핑</p>
      </td></tr>

      <!-- Sections -->
      %s

      <!-- Footer -->
      <tr><td style="padding-top:32px;border-top:1px solid #2d1f42;text-align:center;">
        <p style="margin:0;font-size:12px;color:#9b8bb4;">Over the Algorithm</p>
        <p style="margin:6px 0 0;font-size:11px;color:#4a3d5c;line-height:1.6;">
          알고리즘 너머의 맥락을 전달합니다.<br>
          이 메일은 OTA 브리핑 서비스를 통해 발송되었습니다.
        </p>
      </td></tr>

    </table>
  </td></tr>
</table>
</body>
</html>`, content)
}

func renderEmailSection(title, accentColor string, items []collector.ContextItem, frontendURL string) string {
	var rows strings.Builder
	for i, item := range items {
		borderBottom := "border-bottom:1px solid #2d1f42;"
		if i == len(items)-1 {
			borderBottom = ""
		}

		linkHTML := ""
		if frontendURL != "" && item.Detail != "" {
			linkHTML = fmt.Sprintf(
				`<p style="margin:10px 0 0;"><a href="%s/topic/%s" style="font-size:12px;color:#9b8bb4;text-decoration:none;letter-spacing:0.01em;">자세히 말해주세요 →</a></p>`,
				frontendURL, item.ID,
			)
		}

		rows.WriteString(fmt.Sprintf(`
      <tr><td style="padding:18px 24px;%s">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0">
          <tr>
            <td width="10" style="vertical-align:top;padding-top:6px;">
              <div style="width:6px;height:6px;border-radius:50%%;background-color:%s;"></div>
            </td>
            <td style="padding-left:12px;">
              <p style="margin:0 0 5px;font-size:12px;font-weight:700;color:%s;letter-spacing:0.01em;">%s</p>
              <p style="margin:0;font-size:14px;color:#f5f0ff;line-height:1.7;">%s</p>
              %s
            </td>
          </tr>
        </table>
      </td></tr>`,
			borderBottom, accentColor, accentColor, item.Topic, item.Summary, linkHTML,
		))
	}

	return fmt.Sprintf(`
      <tr><td style="padding-bottom:16px;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#1a1229;border-radius:16px;border:1px solid #2d1f42;">
          <!-- Section header -->
          <tr><td style="padding:16px 24px;border-bottom:1px solid #2d1f42;">
            <p style="margin:0;font-size:11px;font-weight:700;color:%s;letter-spacing:0.1em;text-transform:uppercase;">%s</p>
          </td></tr>
          <!-- Items -->
          %s
        </table>
      </td></tr>
`, accentColor, title, rows.String())
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
