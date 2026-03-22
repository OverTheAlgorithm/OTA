package delivery

import (
	"fmt"
	"strings"
	"time"

	"ota/domain/collector"
	"ota/domain/level"
)

// categoryDisplay maps category keys to Korean labels with emoji for email card badges.
var categoryDisplay = map[string]string{
	"general":       "📰 일반",
	"entertainment": "🎬 연예/오락",
	"business":      "💰 경제/비즈니스",
	"sports":        "⚽ 스포츠",
	"technology":    "💻 기술",
	"science":       "🔬 과학",
	"health":        "🏥 건강",
}

func getCategoryLabel(key string) string {
	if label, ok := categoryDisplay[key]; ok {
		return label
	}
	return "📌 " + key
}

// formatNumber adds comma separators to an integer (e.g. 5000 -> "5,000").
func formatNumber(n int) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

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
			Subject:  "위즈레터",
			TextBody: "오늘은 수집된 소식이 없습니다.",
			HTMLBody: "<p>오늘은 수집된 소식이 없습니다.</p>",
		}
	}

	var preferredItems []collector.ContextItem
	var nonPreferredItems []collector.ContextItem
	for _, item := range items {
		if level.IsPreferredTopic(item.Priority, item.Category, subscriptions) {
			preferredItems = append(preferredItems, item)
		} else {
			nonPreferredItems = append(nonPreferredItems, item)
		}
	}

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
	return fmt.Sprintf("위즈레터 | 오늘의 소식 %d가지", len(items))
}

func generateTextBody(preferred, nonPreferred []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string, msgCtx *MessageContext) string {
	var sections []string

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

	sections = append(sections, "💡 같은 소식이 또 보인다면, 그만큼 오늘도 사람들이 이야기하고 있다는 뜻이에요.")

	return strings.Join(sections, "\n\n")
}

func generateHTMLBody(preferred, nonPreferred []collector.ContextItem, brainCategories []collector.BrainCategory, frontendURL string, levelInfo *UserLevelInfo, msgCtx *MessageContext) string {
	var body strings.Builder

	bcGroups := groupByBrainCategory(preferred)
	for _, bc := range brainCategories {
		if groupItems, ok := bcGroups[bc.Key]; ok {
			body.WriteString(renderBrainCategoryTab(bc.Emoji, bc.Label))
			body.WriteString(renderNewsCards(groupItems, frontendURL, true, msgCtx))
		}
	}
	if ungrouped, ok := bcGroups[""]; ok {
		body.WriteString(renderBrainCategoryTab("📌", "기타"))
		body.WriteString(renderNewsCards(ungrouped, frontendURL, true, msgCtx))
	}

	if len(nonPreferred) > 0 {
		body.WriteString(renderNonPreferredDivider())
		bcGroupsNP := groupByBrainCategory(nonPreferred)
		for _, bc := range brainCategories {
			if groupItems, ok := bcGroupsNP[bc.Key]; ok {
				body.WriteString(renderBrainCategoryTab(bc.Emoji, bc.Label))
				body.WriteString(renderNewsCards(groupItems, frontendURL, false, msgCtx))
			}
		}
		if ungrouped, ok := bcGroupsNP[""]; ok {
			body.WriteString(renderBrainCategoryTab("📌", "기타"))
			body.WriteString(renderNewsCards(ungrouped, frontendURL, false, msgCtx))
		}
	}

	return wrapEmailTemplate(body.String(), frontendURL, levelInfo)
}

func renderNonPreferredDivider() string {
	return `
      <tr><td style="padding:20px 0 8px;">
        <table width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#ffffff;border-radius:8px;border:1px solid #43b9d6;">
          <tr><td style="padding:14px 20px;">
            <p style="margin:0 0 2px;font-size:14px;font-weight:700;color:#43b9d6;">🌱 시야를 넓힐 기회에요</p>
            <p style="margin:0;font-size:12px;color:#231815;">구독하지 않은 주제예요. 읽으면 더 많은 포인트를 얻어요!</p>
          </td></tr>
        </table>
      </td></tr>
`
}

// renderBrainCategoryTab renders a section header with bottom accent border.
func renderBrainCategoryTab(emoji, label string) string {
	return fmt.Sprintf(`
      <tr><td style="padding:24px 0 0;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0">
          <tr><td style="border-bottom:1px solid #dbdade;padding:0;">
            <table cellpadding="0" cellspacing="0" border="0">
              <tr><td style="padding:10px 16px 8px;border-bottom:3px solid #008fb2;">
                <p style="margin:0;font-size:16px;font-weight:500;color:#231815;">%s %s</p>
              </td></tr>
            </table>
          </td></tr>
        </table>
      </td></tr>
`, emoji, label)
}

// renderNewsCards renders a list of news item cards.
func renderNewsCards(items []collector.ContextItem, frontendURL string, preferred bool, msgCtx *MessageContext) string {
	today := time.Now().Format("2006.01.02")
	var rows strings.Builder
	for _, item := range items {
		rows.WriteString(renderNewsCard(item, frontendURL, preferred, msgCtx, today))
	}
	return rows.String()
}

// renderNewsCard renders a single news card with optional thumbnail.
func renderNewsCard(item collector.ContextItem, frontendURL string, preferred bool, msgCtx *MessageContext, date string) string {
	catLabel := getCategoryLabel(item.Category)
	coins := calcCoinsForLink(preferred, msgCtx)
	href := buildTopicLink(frontendURL, item.ID.String(), msgCtx, coins)

	// Coin pill button
	coinPill := renderCoinPill(coins, href)

	// Truncate summary for email
	summary := item.Summary
	if len([]rune(summary)) > 120 {
		summary = string([]rune(summary)[:117]) + "..."
	}

	// Build content cell
	var content strings.Builder
	content.WriteString(fmt.Sprintf(`
                <table width="100%%" cellpadding="0" cellspacing="0" border="0">
                  <tr>
                    <td style="vertical-align:middle;">
                      <p style="margin:0;font-size:12px;font-weight:700;color:#231815;">%s</p>
                    </td>
                    <td align="right" style="vertical-align:middle;">%s</td>
                  </tr>
                </table>
                <p style="margin:6px 0 4px;font-size:11px;font-weight:700;color:#231815;">%s</p>`,
		catLabel, coinPill, date))

	content.WriteString(fmt.Sprintf(`
                <a href="%s" style="text-decoration:none;">
                  <p style="margin:0 0 6px;font-size:15px;font-weight:600;color:#000000;line-height:1.4;">%s</p>
                </a>
                <p style="margin:0;font-size:13px;color:#231815;line-height:1.5;">%s</p>`,
		href, item.Topic, summary))

	// Determine image URL: server image or picsum fallback
	imageURL := fmt.Sprintf("https://picsum.photos/seed/%s/400/250", item.ID.String())
	if item.ImagePath != nil && *item.ImagePath != "" {
		imageURL = fmt.Sprintf("%s/api/v1/images/%s", frontendURL, *item.ImagePath)
	}

	return fmt.Sprintf(`
      <tr><td style="padding:10px 0;">
        <table width="100%%" cellpadding="0" cellspacing="0" border="0" style="border:1px solid #231815;border-radius:8px;">
          <tr>
            <td width="180"
                style="width:180px;background-image:url('%s');background-size:cover;background-position:center;border-radius:7px 0 0 7px;padding:0;margin:0;font-size:0;line-height:0;">
              <a href="%s" style="text-decoration:none;display:block;width:180px;min-height:1px;">&nbsp;</a>
            </td>
            <td valign="top" style="padding:12px 16px;">%s</td>
          </tr>
        </table>
      </td></tr>`, imageURL, href, content.String())
}

// renderCoinPill renders the "+N포인트" or "획득!" pill button.
func renderCoinPill(coins int, href string) string {
	if coins <= 0 {
		return ""
	}
	return fmt.Sprintf(
		`<a href="%s" style="display:inline-block;background-color:#43b9d6;border:1px solid #231815;border-radius:20px;padding:4px 14px;font-size:11px;font-weight:600;color:#231815;text-decoration:none;">+ %d포인트</a>`,
		href, coins,
	)
}

func wrapEmailTemplate(content, frontendURL string, levelInfo *UserLevelInfo) string {
	logoURL := frontendURL + "/wl-logo.png?v=2"
	today := time.Now().Format("2006.01.02")
	dateTitle := today + "의 위즈레터"
	levelCardRow := renderHeaderLevelRow(levelInfo, frontendURL)

	infoText := ""
	if levelInfo != nil {
		infoText = `
      <tr><td style="padding:16px 0 8px;text-align:center;">
        <p style="margin:0;font-size:14px;font-weight:500;color:#231815;line-height:1.6;">
          포인트는 오늘의 소식에서만 모을 수 있어요!<br>
          최신 소식을 확인하고 포인트를 모아보세요
        </p>
      </td></tr>`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#fdf9ee;font-family:'Apple SD Gothic Neo','Malgun Gothic','Noto Sans KR',sans-serif;">
<div style="display:none;font-size:1px;color:#fdf9ee;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;">
  오늘 사람들이 가장 많이 이야기하고 있는 소식을 모아왔어요.
  &#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;&#847;
</div>
<table width="100%%%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#fdf9ee;">
  <tr><td align="center" style="padding:32px 16px 48px;">
    <table width="100%%%%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;">

      <!-- Logo -->
      <tr><td style="padding-bottom:8px;">
        <img src="%s" alt="WizLetter" width="200" style="display:block;">
      </td></tr>

      <!-- Date Title -->
      <tr><td style="padding-bottom:20px;">
        <p style="margin:0;font-size:20px;font-weight:600;color:#231815;letter-spacing:2.5px;">%s</p>
      </td></tr>

      <!-- Level Card -->
      %s

      <!-- Info Text -->
      %s

      <!-- Sections -->
      %s

      <!-- Footer -->
      <tr><td style="padding-top:36px;text-align:center;">
        <table cellpadding="0" cellspacing="0" border="0" align="center">
          <tr><td style="background-color:#5bc2d9;border-radius:11px;padding:4px 16px;">
            <p style="margin:0;font-size:12px;font-weight:700;color:#231815;">WizLetter</p>
          </td></tr>
        </table>
        <p style="margin:14px 0 6px;font-size:11px;color:#231815;line-height:1.7;">
          사업자 등록번호: 000-00-00000 &nbsp;&nbsp; 주소: 서울특별시 강남구 테헤란로 000, 00층 &nbsp;&nbsp; 문의: contact@wizletter.kr
        </p>
        <p style="margin:0 0 6px;font-size:11px;color:#231815;">
          <a href="%s/terms" style="color:#231815;text-decoration:none;">이용약관</a>
          &nbsp;|&nbsp;
          <a href="%s/privacy" style="color:#231815;text-decoration:none;">개인정보처리방침</a>
        </p>
        <p style="margin:0;font-size:11px;color:#231815;">
          &copy; 2026 WizLetter All rights reserved.
        </p>
      </td></tr>

    </table>
  </td></tr>
</table>
</body>
</html>`, logoURL, dateTitle, levelCardRow, infoText, content, frontendURL, frontendURL)
}

// renderHeaderLevelRow returns a full <tr> for the level card.
// Returns an empty string if levelInfo is nil.
// Design matches the frontend LevelCard component: circle with "P", gradient progress bar.
func renderHeaderLevelRow(info *UserLevelInfo, _ string) string {
	if info == nil {
		return ""
	}

	lv := info.Level
	if lv < 1 || lv > 5 {
		lv = 1
	}

	// Progress bar fill percentage
	fillPct := 0
	if info.CoinCap > 0 {
		fillPct = info.TotalCoins * 100 / info.CoinCap
		if fillPct > 100 {
			fillPct = 100
		}
	}

	remaining := info.CoinCap - info.TotalCoins
	if remaining < 0 {
		remaining = 0
	}

	return fmt.Sprintf(`
      <tr><td style="padding-bottom:16px;">
        <table width="100%%%%" cellpadding="0" cellspacing="0" border="0"
               style="background-color:#ffffff;border:2px solid #231815;border-radius:22px;">
          <tr><td style="padding:20px 16px;">
            <table width="100%%%%" cellpadding="0" cellspacing="0" border="0">
              <!-- P Circle (centered on top) -->
              <tr>
                <td align="center" style="padding-bottom:12px;">
                  <!--[if mso]>
                  <v:oval style="width:72px;height:72px;" stroke="true" fill="true" strokecolor="#231815" strokeweight="3px">
                    <v:fill color="#d4eff5"/>
                    <v:textbox inset="0,0,0,0" style="mso-fit-shape-to-text:false;"><center style="font-size:30px;font-weight:700;color:#231815;">P</center></v:textbox>
                  </v:oval>
                  <![endif]-->
                  <!--[if !mso]><!-->
                  <div style="width:72px;height:72px;border-radius:36px;border:3px solid #231815;background-color:#d4eff5;text-align:center;line-height:72px;font-size:30px;font-weight:700;color:#231815;">P</div>
                  <!--<![endif]-->
                </td>
              </tr>
              <!-- Level & Points (centered) -->
              <tr>
                <td align="center">
                  <p style="margin:0;font-size:18px;font-weight:700;color:#231815;line-height:1.2;">Lv.%d</p>
                  <p style="margin:4px 0 0;">
                    <span style="font-size:28px;font-weight:700;color:#231815;">%s</span>
                    <span style="font-size:14px;font-weight:700;color:#231815;"> 포인트</span>
                  </p>
                </td>
              </tr>
              <!-- Progress Bar (full width) -->
              <tr>
                <td style="padding-top:12px;">
                  <table width="100%%%%" cellpadding="0" cellspacing="0" border="0"
                         style="background-color:#e8f4fd;border-radius:7px;border:1px solid #c0c0c0;">
                    <tr>
                      <td width="%d%%%%" style="background-color:#43b9d6;height:14px;border-radius:7px;font-size:1px;line-height:14px;">&nbsp;</td>
                      <td style="height:14px;font-size:1px;line-height:14px;">&nbsp;</td>
                    </tr>
                  </table>
                </td>
              </tr>
              <!-- Level-up text & ratio -->
              <tr>
                <td style="padding-top:8px;">
                  <table width="100%%%%" cellpadding="0" cellspacing="0" border="0">
                    <tr>
                      <td style="font-size:13px;font-weight:700;color:#231815;white-space:nowrap;">%s 포인트를 더 모으면 레벨업!</td>
                      <td align="right" style="font-size:13px;font-weight:700;color:#231815;white-space:nowrap;">%s / %s</td>
                    </tr>
                  </table>
                </td>
              </tr>
            </table>
          </td></tr>
        </table>
      </td></tr>`,
		lv,
		formatNumber(info.TotalCoins),
		fillPct,
		formatNumber(remaining),
		formatNumber(info.TotalCoins), formatNumber(info.CoinCap),
	)
}

// calcCoinsForLink returns the pre-calculated coin value for embedding in links.
func calcCoinsForLink(preferred bool, msgCtx *MessageContext) int {
	if msgCtx == nil {
		return 0
	}
	return level.CalcCoins(preferred)
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

// buildCoinsLabel returns the " +X코인" HTML span for the given coin value.
func buildCoinsLabel(coins int) string {
	if coins <= 0 {
		return ""
	}
	return fmt.Sprintf(`  <span style="font-size:11px;color:#43b9d6;font-weight:700;">+%d포인트</span>`, coins)
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
			coins := calcCoinsForLink(preferred, msgCtx)
			href := buildTopicLink(frontendURL, item.ID.String(), msgCtx, coins)
			coinsLabel := ""
			if coins > 0 {
				coinsLabel = fmt.Sprintf(" +%d포인트", coins)
			}
			line += fmt.Sprintf("\n   👉 %d개의 추가 정보: %s%s", len(item.Details), href, coinsLabel)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
