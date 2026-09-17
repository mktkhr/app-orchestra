package usecase

import (
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// withoutOptionListing strips from an ask's question text the lines that
// merely list the options the answer already carries. The browser renders
// options as choices under the question (AC-F-103), so a question the model
// wrote as 「…どれをお探しでしょうか？\n- 引当済 (allocated)\n- 出荷準備完了
// (staged)…」 showed every option twice (reported 2026-09-17). A line is a
// listing when it starts with a bullet or an ordinal and names an option's
// value or label, or when it is nothing but an option's value or label.
// The prose stays as the model wrote it; when nothing but listing lines
// remain, the original text is returned unchanged rather than an empty
// question.
func withoutOptionListing(question string, options []domain.Option) string {
	if len(options) == 0 {
		return question
	}

	var kept []string
	for line := range strings.SplitSeq(question, "\n") {
		if !isOptionListingLine(line, options) {
			kept = append(kept, line)
		}
	}

	trimmed := strings.TrimSpace(strings.Join(kept, "\n"))
	if trimmed == "" {
		return question
	}

	return trimmed
}

// listingMarkers are the bullets a model puts before a listed option.
func listingMarkers() []string {
	return []string{"- ", "* ", "• ", "・", "-\t"}
}

func isOptionListingLine(line string, options []domain.Option) bool {
	text := strings.TrimSpace(line)
	if text == "" {
		return false
	}

	bulleted := false
	for _, marker := range listingMarkers() {
		if strings.HasPrefix(text, marker) {
			bulleted = true
			text = strings.TrimSpace(strings.TrimPrefix(text, marker))

			break
		}
	}

	if !bulleted {
		text = strings.TrimLeft(text, "0123456789.)）、 ")
		bulleted = text != strings.TrimSpace(line)
	}

	for _, option := range options {
		if text == option.Value || text == option.Label {
			return true
		}

		if bulleted && (strings.Contains(text, option.Value) || (option.Label != "" && strings.Contains(text, option.Label))) {
			return true
		}
	}

	return false
}
