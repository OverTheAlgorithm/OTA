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
// - Always include items from "top" and "brief" categories (universal topics)
// - Append items from subscribed categories
// - Groups items by brain_category (action-oriented labels) instead of raw category
// - brainCategories provides the display metadata (emoji, label, color, order)
// - frontendURL is used to generate detail links per item.
//   Pass empty string to omit links (e.g. in tests or when URL is unavailable).
func FormatMessage(items []collector.ContextItem, subscriptions []string, brainCategories []collector.BrainCategory, frontendURL string, levelInfo *UserLevelInfo) FormattedMessage {
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

	// Filter items: always include "top", "brief", "over_the_algorithm", plus subscribed categories
	var selectedItems []collector.ContextItem
	for _, item := range items {
		if item.Category == "top" || item.Category == "brief" || item.BrainCategory == "over_the_algorithm" || subSet[item.Category] {
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
	textBody := generateTextBody(selectedItems, brainCategories, frontendURL)
	htmlBody := generateHTMLBody(selectedItems, brainCategories, frontendURL, levelInfo)

	return FormattedMessage{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}

func generateSubject(items []collector.ContextItem) string {
	universalCount := 0
	for _, item := range items {
		if item.Category == "top" || item.Category == "brief" {
			universalCount++
		}
	}

	if universalCount == len(items) {
		return fmt.Sprintf("오늘의 맥락 %d가지", len(items))
	}

	return fmt.Sprintf("오늘의 맥락 %d가지 (구독 주제 포함)", len(items))
}

func generateTextBody(items []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string) string {
	var sections []string

	bcGroups := groupByBrainCategory(items)

	// Render in brain category display_order
	for _, bc := range brainCategories {
		if groupItems, ok := bcGroups[bc.Key]; ok {
			header := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
			sections = append(sections, header+"\n"+formatItemsAsText(groupItems, frontendURL))
		}
	}

	// Items without brain_category (legacy/unmapped)
	if ungrouped, ok := bcGroups[""]; ok {
		sections = append(sections, "📌 기타\n"+formatItemsAsText(ungrouped, frontendURL))
	}

	return strings.Join(sections, "\n\n")
}

func generateHTMLBody(items []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string, levelInfo *UserLevelInfo) string {
	bcGroups := groupByBrainCategory(items)

	var body strings.Builder

	// Render in brain category display_order
	for _, bc := range brainCategories {
		if groupItems, ok := bcGroups[bc.Key]; ok {
			sectionTitle := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
			body.WriteString(renderEmailSection(sectionTitle, bc.AccentColor, groupItems, frontendURL))
		}
	}

	// Items without brain_category (legacy/unmapped)
	if ungrouped, ok := bcGroups[""]; ok {
		body.WriteString(renderEmailSection("📌 기타", "#9b8bb4", ungrouped, frontendURL))
	}

	return wrapEmailTemplate(body.String(), frontendURL, levelInfo)
}

func wrapEmailTemplate(content, frontendURL string, levelInfo *UserLevelInfo) string {
	logoURL := frontendURL + "/OTA_logo.png"
	headerLevelCell := renderHeaderLevelCell(levelInfo, frontendURL)
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

      <!-- Header: Logo + Level Card -->
      <tr><td style="padding-bottom:24px;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0">
          <tr>
            <!-- Logo -->
            <td width="160" style="vertical-align:middle;text-align:center;">
              <img src="%s" alt="OTA" width="150" style="display:block;margin:0 auto;max-width:150px;">
              <p style="margin:6px 0 0;font-size:12px;color:#9b8bb4;">오늘의 맥락 브리핑</p>
            </td>
            %s
          </tr>
        </table>
      </td></tr>

      <!-- Sections -->
      %s

      <!-- Footer -->
      <tr><td style="padding-top:32px;border-top:1px solid #2d1f42;text-align:center;">
        <p style="margin:0;font-size:11px;color:#4a3d5c;line-height:1.6;">
          알고리즘 너머의 맥락을 전달합니다.<br>
          이 메일은 OTA 브리핑 서비스를 통해 발송되었습니다.
        </p>
      </td></tr>

    </table>
  </td></tr>
</table>
</body>
</html>`, logoURL, headerLevelCell, content)
}

// renderHeaderLevelCell returns the <td> for the level card in the header row.
// Returns an empty <td> if levelInfo is nil (logo takes full width).
func renderHeaderLevelCell(info *UserLevelInfo, frontendURL string) string {
	if info == nil {
		return ""
	}

	lv := info.Level
	if lv < 1 || lv > 5 {
		lv = 1
	}
	imgURL := fmt.Sprintf("%s/rainbow_lv%d.png", frontendURL, lv)

	var progressText string
	var progressPercent int
	if info.PointsToNext == 0 {
		progressText = `<p style="margin:0 0 5px;font-size:11px;color:#f0923b;font-weight:700;">MAX</p>`
		progressPercent = 100
	} else {
		progressText = fmt.Sprintf(
			`<p style="margin:0 0 5px;font-size:11px;color:#9b8bb4;">%d / %d</p>`,
			info.CurrentProgress, info.PointsToNext,
		)
		progressPercent = int(float64(info.CurrentProgress) / float64(info.PointsToNext) * 100)
	}

	return fmt.Sprintf(`
            <!-- Level Card -->
            <td style="padding-left:12px;vertical-align:middle;">
              <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#1a1229;border-radius:16px;border:1px solid #2d1f42;">
                <tr><td style="padding:10px 16px;border-bottom:1px solid #2d1f42;">
                  <p style="margin:0;font-size:10px;font-weight:700;color:#5ba4d9;letter-spacing:0.08em;">🌈 나의 레벨</p>
                </td></tr>
                <tr><td style="padding:12px 16px;">
                  <table width="100%%" cellpadding="0" cellspacing="0" border="0">
                    <tr>
                      <td width="72" style="vertical-align:middle;">
                        <img src="%s" alt="Lv.%d" width="72" style="display:block;">
                      </td>
                      <td style="padding-left:10px;vertical-align:middle;">
                        <p style="margin:0 0 2px;font-size:17px;font-weight:700;color:#f5f0ff;">Lv.%d</p>
                        %s
                        <div style="width:100%%;height:5px;background-color:#2d1f42;border-radius:3px;overflow:hidden;margin-bottom:6px;">
                          <div style="width:%d%%;height:100%%;background:linear-gradient(to right,#5ba4d9,#7bc67e);border-radius:3px;"></div>
                        </div>
                        <p style="margin:0;font-size:11px;color:#9b8bb4;">%s</p>
                        <p style="margin:4px 0 0;font-size:10px;color:#4a3d5c;">🌈 토픽을 읽으면 포인트가 쌓여요</p>
                      </td>
                    </tr>
                  </table>
                </td></tr>
              </table>
            </td>`,
		imgURL, lv, lv, progressText, progressPercent, info.Description,
	)
}

func renderEmailSection(title, accentColor string, items []collector.ContextItem, frontendURL string) string {
	var rows strings.Builder
	for i, item := range items {
		borderBottom := "border-bottom:1px solid #2d1f42;"
		if i == len(items)-1 {
			borderBottom = ""
		}

		buzzHTML := ""
		if item.BuzzScore > 0 {
			buzzHTML = fmt.Sprintf(
				`<p style="margin:0 0 4px;font-size:11px;color:#e84d3d;font-weight:700;letter-spacing:0.01em;">🔥 화제도 %d</p>`,
				item.BuzzScore,
			)
		}

		linkHTML := ""
		if frontendURL != "" && len(item.Details) > 0 {
			linkHTML = fmt.Sprintf(
				`<p style="margin:10px 0 0;"><a href="%s/topic/%s" style="font-size:12px;color:#9b8bb4;text-decoration:none;letter-spacing:0.01em;">%d개의 추가 정보가 있어요 →</a></p>`,
				frontendURL, item.ID, len(item.Details),
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
              %s
              <p style="margin:0 0 6px;font-size:14px;font-weight:700;color:#f5f0ff;letter-spacing:-0.01em;">%s</p>
              <p style="margin:0;font-size:13px;color:#d4cee0;line-height:1.7;">%s</p>
              %s
            </td>
          </tr>
        </table>
      </td></tr>`,
			borderBottom, accentColor, buzzHTML, item.Topic, item.Summary, linkHTML,
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

func groupByBrainCategory(items []collector.ContextItem) map[string][]collector.ContextItem {
	groups := make(map[string][]collector.ContextItem)
	for _, item := range items {
		groups[item.BrainCategory] = append(groups[item.BrainCategory], item)
	}
	return groups
}


func formatItemsAsText(items []collector.ContextItem, frontendURL string) string {
	var lines []string
	for i, item := range items {
		buzzStr := ""
		if item.BuzzScore > 0 {
			buzzStr = fmt.Sprintf(" 🔥화제도 %d", item.BuzzScore)
		}
		line := fmt.Sprintf("%d. %s%s: %s", i+1, item.Topic, buzzStr, item.Summary)
		if frontendURL != "" && len(item.Details) > 0 {
			line += fmt.Sprintf("\n   👉 %d개의 추가 정보: %s/topic/%s", len(item.Details), frontendURL, item.ID)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}


