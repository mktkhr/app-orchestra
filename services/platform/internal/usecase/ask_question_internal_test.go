package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func statusOptions() []domain.Option {
	return []domain.Option{
		{Value: "allocated", Label: "引当済"},
		{Value: "staged", Label: "出荷準備完了"},
		{Value: "quarantined", Label: "検品保留"},
		{Value: "consigned", Label: "預託在庫"},
	}
}

func TestWithoutOptionListingDropsTheBulletedOptions(t *testing.T) {
	question := "「破損した在庫」をお探しのようですが、在庫ステータスには以下の選択肢があります。どれをお探しでしょうか？\n" +
		"- 引当済 (allocated)\n- 出荷準備完了 (staged)\n- 検品保留 (quarantined)\n- 預託在庫 (consigned)"

	got := withoutOptionListing(question, statusOptions())

	assert.Equal(t, "「破損した在庫」をお探しのようですが、在庫ステータスには以下の選択肢があります。どれをお探しでしょうか？", got)
}

func TestWithoutOptionListingDropsNumberedAndBareOptionLines(t *testing.T) {
	question := "どれですか？\n1. 引当済\n2. 出荷準備完了\nquarantined\n預託在庫"

	got := withoutOptionListing(question, statusOptions())

	assert.Equal(t, "どれですか？", got)
}

func TestWithoutOptionListingKeepsProseThatMentionsAnOption(t *testing.T) {
	question := "検品保留のことでしょうか？それとも別のステータスですか？"

	got := withoutOptionListing(question, statusOptions())

	assert.Equal(t, question, got)
}

func TestWithoutOptionListingKeepsTheOriginalWhenOnlyListingLinesRemain(t *testing.T) {
	question := "- 引当済\n- 出荷準備完了"

	got := withoutOptionListing(question, statusOptions())

	assert.Equal(t, question, got)
}

func TestWithoutOptionListingWithoutOptionsIsIdentity(t *testing.T) {
	question := "どの注文ですか？\n- 受注\n- 発注"

	assert.Equal(t, question, withoutOptionListing(question, nil))
}
