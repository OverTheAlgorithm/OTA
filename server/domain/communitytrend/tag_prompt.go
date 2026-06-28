package communitytrend

import (
	"fmt"
	"strings"
)

// BuildTagPrompt assembles the single-pass analysis prompt: topic tagging +
// meme matching + meme-candidate discovery. Rules enforce conservative tagging
// and precise naming (decisions.md D-002, technical-spec §8).
func BuildTagPrompt(in TaggerInput) string {
	var b strings.Builder

	b.WriteString("당신은 한국 커뮤니티의 '논제'를 분석하는 도구입니다. ")
	b.WriteString("아래 게시글 제목 묶음을 보고, 어떤 논제가 다뤄졌는지 태그로 분류하세요.\n\n")

	b.WriteString("# 규칙 (반드시 준수)\n")
	b.WriteString("1. 기존 태그를 우선 사용하세요. 새로운 태그를 제안하는 것은 매우 보수적이어야 합니다. 정말로 알맞은 기존 태그가 전혀 없고, 다수의 게시글에서 반복되는 중요 논제일 때만 새 태그를 제안(is_new=true)하세요.\n")
	b.WriteString("2. 게시글 제목들을 읽고 그 주제가 분류 체계의 어떤 태그에 속하는지 중립적으로 전부 분류하세요. AI가 임의로 글을 거르지 않고, 해당하는 글이 하나라도 있다면 post_indices에 그 글 번호를 매핑해야 합니다.\n")
	b.WriteString("3. 태그 이름은 정밀해야 합니다. 예: '우파'가 아니라 '우파 지지', '남성'이 아니라 '남성 인권'.\n")
	b.WriteString("4. 글 본문을 인용하거나 재현하지 마세요. 논제(주제)만 추출합니다.\n")
	b.WriteString("5. 각 분류에 해당하는 게시글 번호(1부터 시작)를 post_indices 배열에 수집하세요.\n")
	b.WriteString("6. 확정 밈 목록에 있는 표현이 보이면 meme_matches에 매칭된 글 번호를 post_indices에 넣으세요(별칭 포함).\n")
	b.WriteString("7. 기존 태그·일반어·밈목록·금지목록에 없는데 여러 번 반복되는 신조어는 meme_candidates의 post_indices에 분류하세요.\n\n")

	b.WriteString("# 기존 태그 분류체계 (axis_key: 태그명)\n")
	if len(in.Taxonomy) == 0 {
		b.WriteString("(없음)\n")
	}
	for _, t := range in.Taxonomy {
		b.WriteString(fmt.Sprintf("- [%s] %s (id=%d)\n", t.AxisKey, t.Name, t.ID))
	}
	b.WriteString("\n")

	b.WriteString("# 확정 밈 목록 (별칭 포함)\n")
	if len(in.Memes) == 0 {
		b.WriteString("(없음)\n")
	}
	for _, m := range in.Memes {
		if len(m.Aliases) > 0 {
			b.WriteString(fmt.Sprintf("- %s (id=%d, 별칭: %s)\n", m.Name, m.ID, strings.Join(m.Aliases, ", ")))
		} else {
			b.WriteString(fmt.Sprintf("- %s (id=%d)\n", m.Name, m.ID))
		}
	}
	b.WriteString("\n")

	if len(in.Blacklist) > 0 {
		b.WriteString("# 밈 후보 금지 목록 (다시 제안하지 마세요)\n")
		b.WriteString(strings.Join(in.Blacklist, ", "))
		b.WriteString("\n\n")
	}

	b.WriteString("# 분석할 게시글 제목\n")
	for i, title := range in.Titles {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
	}
	b.WriteString("\n")

	b.WriteString("# 출력 (JSON만, 설명 없이)\n")
	b.WriteString(`{"tags":[{"tag_id":0,"name":"태그명","axis_key":"축키","post_indices":[1,2],"is_new":false,"new_axis_key":""}],`)
	b.WriteString(`"meme_matches":[{"meme_id":1,"name":"밈명","post_indices":[3]}],`)
	b.WriteString(`"meme_candidates":[{"expression":"신조어","post_indices":[4,5]}]}`)
	b.WriteString("\n기존 태그는 tag_id를 채우고 is_new=false. 새 태그는 tag_id=0, is_new=true, new_axis_key 지정.\n")

	return b.String()
}
