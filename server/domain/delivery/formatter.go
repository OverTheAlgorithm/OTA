package delivery

import (
	"fmt"
	"strings"

	"ota/domain/collector"
	"ota/domain/level"
)


// FormatMessage creates a personalized message from context items.
// This is a pure function - no side effects, completely testable.
//
// Rules:
//   - "preferred" items: category is "top", "brief", or in user's subscriptions
//   - "non-preferred" items: everything else (shown in a separate section below)
//   - All items are always included — preferred section first, then non-preferred
//   - brainCategories provides the display metadata (emoji, label, color, order)
//   - frontendURL is used to generate detail links per item.
//   - msgCtx carries uid/rid for link tracking and days-since-earn for point display.
//     Pass nil for msgCtx to omit personalized links (e.g. welcome emails).
func FormatMessage(items []collector.ContextItem, subscriptions []string, brainCategories []collector.BrainCategory, frontendURL string, levelInfo *UserLevelInfo, msgCtx *MessageContext) FormattedMessage {
	if len(items) == 0 {
		return FormattedMessage{
			Subject:  "오늘의 맥락",
			TextBody: "오늘은 수집된 맥락이 없습니다.",
			HTMLBody: "<p>오늘은 수집된 맥락이 없습니다.</p>",
		}
	}

	// Build subscription set for fast lookup - NOT NEEDED ANYMORE BUT WE PASS SLICE DIRECTLY

	// Split into preferred and non-preferred sections
	var preferredItems []collector.ContextItem
	var nonPreferredItems []collector.ContextItem
	for _, item := range items {
		if level.IsPreferredCategory(item.Category, subscriptions) {
			preferredItems = append(preferredItems, item)
		} else {
			nonPreferredItems = append(nonPreferredItems, item)
		}
	}

	// If nothing is preferred (edge case: no top/brief items and no subscriptions),
	// treat all items as preferred to avoid an empty first section.
	if len(preferredItems) == 0 {
		preferredItems = items
		nonPreferredItems = nil
	}

	subject := generateSubject(items)
	textBody := generateTextBody(preferredItems, nonPreferredItems, brainCategories, frontendURL, msgCtx)
	htmlBody := generateHTMLBody(preferredItems, nonPreferredItems, brainCategories, frontendURL, levelInfo, msgCtx)

	return FormattedMessage{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}
}



func generateSubject(items []collector.ContextItem) string {
	return fmt.Sprintf("오늘의 맥락 %d가지", len(items))
}

func generateTextBody(preferred, nonPreferred []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string, msgCtx *MessageContext) string {
	var sections []string

	// Preferred section
	bcGroups := groupByBrainCategory(preferred)
	for _, bc := range brainCategories {
		if groupItems, ok := bcGroups[bc.Key]; ok {
			header := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
			sections = append(sections, header+"\n"+formatItemsAsText(groupItems, frontendURL, true, msgCtx))
		}
	}
	if ungrouped, ok := bcGroups[""]; ok {
		sections = append(sections, "📌 기타\n"+formatItemsAsText(ungrouped, frontendURL, true, msgCtx))
	}

	// Non-preferred section
	if len(nonPreferred) > 0 {
		sections = append(sections, "🌱 시야를 넓힐 기회에요")
		bcGroupsNP := groupByBrainCategory(nonPreferred)
		for _, bc := range brainCategories {
			if groupItems, ok := bcGroupsNP[bc.Key]; ok {
				header := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
				sections = append(sections, header+"\n"+formatItemsAsText(groupItems, frontendURL, false, msgCtx))
			}
		}
		if ungrouped, ok := bcGroupsNP[""]; ok {
			sections = append(sections, "📌 기타\n"+formatItemsAsText(ungrouped, frontendURL, false, msgCtx))
		}
	}

	return strings.Join(sections, "\n\n")
}

func generateHTMLBody(preferred, nonPreferred []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string, levelInfo *UserLevelInfo, msgCtx *MessageContext) string {
	var body strings.Builder

	// Preferred sections
	bcGroups := groupByBrainCategory(preferred)
	for _, bc := range brainCategories {
		if groupItems, ok := bcGroups[bc.Key]; ok {
			sectionTitle := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
			body.WriteString(renderEmailSection(sectionTitle, bc.AccentColor, groupItems, frontendURL, true, msgCtx))
		}
	}
	if ungrouped, ok := bcGroups[""]; ok {
		body.WriteString(renderEmailSection("📌 기타", "#6b8db5", ungrouped, frontendURL, true, msgCtx))
	}

	// Non-preferred sections with divider
	if len(nonPreferred) > 0 {
		body.WriteString(renderNonPreferredDivider())
		bcGroupsNP := groupByBrainCategory(nonPreferred)
		for _, bc := range brainCategories {
			if groupItems, ok := bcGroupsNP[bc.Key]; ok {
				sectionTitle := fmt.Sprintf("%s %s", bc.Emoji, bc.Label)
				body.WriteString(renderEmailSection(sectionTitle, bc.AccentColor, groupItems, frontendURL, false, msgCtx))
			}
		}
		if ungrouped, ok := bcGroupsNP[""]; ok {
			body.WriteString(renderEmailSection("📌 기타", "#6b8db5", ungrouped, frontendURL, false, msgCtx))
		}
	}

	return wrapEmailTemplate(body.String(), frontendURL, levelInfo)
}

// renderNonPreferredDivider returns the HTML row separating preferred and non-preferred sections.
func renderNonPreferredDivider() string {
	return `
      <tr><td style="padding-bottom:16px;">
        <table width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f0f7ff;border-radius:16px;border:1px solid #7bc67e;">
          <tr><td style="padding:16px 24px;">
            <p style="margin:0 0 4px;font-size:14px;font-weight:700;color:#7bc67e;">🌱 시야를 넓힐 기회에요</p>
            <p style="margin:0;font-size:12px;color:#6b8db5;">구독하지 않은 주제예요. 읽으면 더 많은 포인트를 얻어요!</p>
          </td></tr>
        </table>
      </td></tr>
`
}

func wrapEmailTemplate(content, frontendURL string, levelInfo *UserLevelInfo) string {
	logoURL := frontendURL + "/OTA_logo.png"
	levelCardRow := renderHeaderLevelRow(levelInfo, frontendURL)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:white;font-family:'Apple SD Gothic Neo','Malgun Gothic','Noto Sans KR',sans-serif;">
<div style="display:none;font-size:1px;color:white;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;">
  오늘 사람들이 가장 많이 이야기하고 있는 소식을 모아왔어요.
  &#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;
</div>
<table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:white;">
  <tr><td align="center" style="padding:32px 16px 48px;">
    <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;">

      <!-- Header Row 1: Logo (small, left-aligned, ~60px) -->
      <tr><td style="height:60px;vertical-align:middle;padding-bottom:16px;">
        <table cellpadding="0" cellspacing="0" border="0">
          <tr>
            <td style="vertical-align:middle;">
              <img src="%s" alt="OTA" height="32" style="display:block;">
            </td>
            <td style="padding-left:10px;vertical-align:middle;">
              <p style="margin:0;font-size:16px;font-weight:700;color:#1e3a5f;letter-spacing:-0.01em;">Over the Algorithm</p>
              <p style="margin:2px 0 0;font-size:11px;color:#6b8db5;">오늘의 맥락 브리핑</p>
            </td>
          </tr>
        </table>
      </td></tr>

      <!-- Header Row 2: Level Card -->
      %s

      <!-- Sections -->
      %s

      <!-- Footer -->
      <tr><td style="padding-top:32px;border-top:1px solid #d4e6f5;text-align:center;">
        <p style="margin:0;font-size:11px;color:#a8bcc9;line-height:1.6;">
          알고리즘 너머의 맥락을 전달합니다.<br>
          이 메일은 OTA 브리핑 서비스를 통해 발송되었습니다.
        </p>
      </td></tr>

    </table>
  </td></tr>
</table>
</body>
</html>`, logoURL, levelCardRow, content)
}

// renderHeaderLevelRow returns a full <tr> for the level card placed below the logo row.
// Returns an empty string if levelInfo is nil.
func renderHeaderLevelRow(info *UserLevelInfo, frontendURL string) string {
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
			`<p style="margin:0 0 5px;font-size:11px;color:#6b8db5;">%d / %d</p>`,
			info.CurrentProgress, info.PointsToNext,
		)
		progressPercent = int(float64(info.CurrentProgress) / float64(info.PointsToNext) * 100)
	}

	return fmt.Sprintf(`
      <!-- Level Card -->
      <tr><td style="padding-bottom:24px;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f0f7ff;border-radius:16px;border:1px solid #d4e6f5;">
          <tr><td style="padding:10px 16px;border-bottom:1px solid #d4e6f5;">
            <p style="margin:0;font-size:10px;font-weight:700;color:#26b0ff;letter-spacing:0.08em;">🌈 나의 레벨</p>
          </td></tr>
          <tr><td style="padding:12px 16px;">
            <table width="100%%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td width="72" style="vertical-align:middle;">
                  <img src="%s" alt="Lv.%d" width="72" style="display:block;">
                </td>
                <td style="padding-left:10px;vertical-align:middle;">
                  <p style="margin:0 0 2px;font-size:17px;font-weight:700;color:#1e3a5f;">Lv.%d</p>
                  %s
                  <div style="width:100%%;height:5px;background-color:#d4e6f5;border-radius:3px;overflow:hidden;margin-bottom:6px;">
                    <div style="width:%d%%;height:100%%;background:linear-gradient(to right,#26b0ff,#7bc67e);border-radius:3px;"></div>
                  </div>
                  <p style="margin:0;font-size:11px;color:#6b8db5;">%s</p>
                  <p style="margin:4px 0 0;font-size:10px;color:#a8bcc9;">🌈 토픽을 읽으면 포인트가 쌓여요</p>
                </td>
              </tr>
            </table>
          </td></tr>
        </table>
      </td></tr>`,
		imgURL, lv, lv, progressText, progressPercent, info.Description,
	)
}

func renderEmailSection(title, accentColor string, items []collector.ContextItem, frontendURL string, preferred bool, msgCtx *MessageContext) string {
	var rows strings.Builder
	for i, item := range items {
		borderBottom := "border-bottom:1px solid #d4e6f5;"
		if i == len(items)-1 {
			borderBottom = ""
		}

		buzzHTML := ""
		if item.BuzzScore > 0 {
			buzzHTML = fmt.Sprintf(
				`<p style="margin:0 0 4px;font-size:11px;color:#ff5442;font-weight:700;letter-spacing:0.01em;">🔥 화제도 %d</p>`,
				item.BuzzScore,
			)
		}

		linkHTML := ""
		if frontendURL != "" && len(item.Details) > 0 {
			pts := calcPointsForLink(preferred, msgCtx)
			href := buildTopicLink(frontendURL, item.ID.String(), msgCtx, pts)
			pointsLabel := buildPointsLabelFromPts(pts)
			linkHTML = fmt.Sprintf(
				`<p style="margin:10px 0 0;"><a href="%s" style="font-size:12px;color:#6b8db5;text-decoration:none;letter-spacing:0.01em;">%d개의 추가 정보가 있어요 →%s</a></p>`,
				href, len(item.Details), pointsLabel,
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
              <p style="margin:0 0 6px;font-size:14px;font-weight:700;color:#1e3a5f;letter-spacing:-0.01em;">%s</p>
              <p style="margin:0;font-size:13px;color:#6b8db5;line-height:1.7;">%s</p>
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
        <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f0f7ff;border-radius:16px;border:1px solid #d4e6f5;">
          <!-- Section header -->
          <tr><td style="padding:16px 24px;border-bottom:1px solid #d4e6f5;">
            <p style="margin:0;font-size:14px;font-weight:700;color:%s;letter-spacing:0.1em;text-transform:uppercase;">%s</p>
          </td></tr>
          <!-- Items -->
          %s
        </table>
      </td></tr>
`, accentColor, title, rows.String())
}

// calcPointsForLink returns the pre-calculated points value for embedding in links.
// Returns 0 if msgCtx is nil (no point tracking).
func calcPointsForLink(preferred bool, msgCtx *MessageContext) int {
	if msgCtx == nil {
		return 0
	}
	return level.CalcPoints(preferred)
}

// buildTopicLink constructs the topic detail URL with optional uid/rid/pts tracking params.
func buildTopicLink(frontendURL, itemID string, msgCtx *MessageContext, pts int) string {
	base := fmt.Sprintf("%s/topic/%s", frontendURL, itemID)
	if msgCtx == nil || msgCtx.UserID == "" {
		return base
	}
	link := fmt.Sprintf("%s?uid=%s&rid=%s", base, msgCtx.UserID, msgCtx.RunID)
	if pts > 0 {
		link += fmt.Sprintf("&pts=%d", pts)
	}
	return link
}

// buildPointsLabelFromPts returns the " +Xpt" HTML span for the given points value.
func buildPointsLabelFromPts(pts int) string {
	if pts <= 0 {
		return ""
	}
	return fmt.Sprintf(`  <span style="font-size:11px;color:#7bc67e;font-weight:700;">+%dpt</span>`, pts)
}

func groupByBrainCategory(items []collector.ContextItem) map[string][]collector.ContextItem {
	groups := make(map[string][]collector.ContextItem)
	for _, item := range items {
		groups[item.BrainCategory] = append(groups[item.BrainCategory], item)
	}
	return groups
}

func formatItemsAsText(items []collector.ContextItem, frontendURL string, preferred bool, msgCtx *MessageContext) string {
	var lines []string
	for i, item := range items {
		buzzStr := ""
		if item.BuzzScore > 0 {
			buzzStr = fmt.Sprintf(" 🔥화제도 %d", item.BuzzScore)
		}
		line := fmt.Sprintf("%d. %s%s: %s", i+1, item.Topic, buzzStr, item.Summary)
		if frontendURL != "" && len(item.Details) > 0 {
			pts := calcPointsForLink(preferred, msgCtx)
			href := buildTopicLink(frontendURL, item.ID.String(), msgCtx, pts)
			ptsLabel := ""
			if pts > 0 {
				ptsLabel = fmt.Sprintf(" +%dpt", pts)
			}
			line += fmt.Sprintf("\n   👉 %d개의 추가 정보: %s%s", len(item.Details), href, ptsLabel)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
