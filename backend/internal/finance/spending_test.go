package finance

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubSpendingRepository struct {
	summary     SpendingSummary
	err         error
	seenUserID  int64
	seenQuery   spendingSummaryQuery
	calledCount int
}

func (s *stubSpendingRepository) SpendingSummary(_ context.Context, userID int64, query spendingSummaryQuery) (SpendingSummary, error) {
	s.calledCount++
	s.seenUserID = userID
	s.seenQuery = query
	return s.summary, s.err
}

// TestSpendingServiceValidatesPeriodAndAddsMetadata は、UTC期間を内部境界へ変換してresponse契約を補完することを確認する。
// 必要な理由: 日付上端をinclusiveと誤解した集計漏れを防ぎ、frontendが期間とtimezoneを推測しないようにするため。
func TestSpendingServiceValidatesPeriodAndAddsMetadata(t *testing.T) {
	repository := &stubSpendingRepository{summary: SpendingSummary{Currencies: []SpendingCurrency{}}}
	service := NewSpendingService(repository)

	summary, err := service.SpendingSummary(context.Background(), 42, SpendingSummaryRequest{
		DateFrom: "2026-08-01",
		DateTo:   "2026-08-31",
	})
	if err != nil {
		t.Fatalf("get spending summary: %v", err)
	}
	wantFrom := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	wantToExclusive := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	if repository.seenUserID != 42 || !repository.seenQuery.DateFrom.Equal(wantFrom) || !repository.seenQuery.DateToExclusive.Equal(wantToExclusive) {
		t.Fatalf("expected owner and inclusive UTC period, got owner=%d query=%#v", repository.seenUserID, repository.seenQuery)
	}
	if summary.DateFrom != "2026-08-01" || summary.DateTo != "2026-08-31" || summary.Timezone != "UTC" || summary.Currencies == nil {
		t.Fatalf("unexpected response metadata: %#v", summary)
	}
}

// TestSpendingServiceRejectsInvalidPeriod は、不正または逆転した期間をrepositoryより前で拒否することを確認する。
// 必要な理由: 曖昧な日付をDBへ渡さず、全clientへ同じ400相当のerrorを返すため。
func TestSpendingServiceRejectsInvalidPeriod(t *testing.T) {
	cases := []SpendingSummaryRequest{
		// 開始日はゼロ埋めしたYYYY-MM-DDを必須とする。
		{DateFrom: "2026-8-01", DateTo: "2026-08-31"},
		// 終了日は実在するUTC暦日を必須とする。
		{DateFrom: "2026-08-01", DateTo: "2026-08-32"},
		// 空の期間境界を既定値へ暗黙変換しない。
		{DateFrom: "", DateTo: "2026-08-31"},
		{DateFrom: "2026-08-01", DateTo: ""},
		// 終了日が開始日より前の期間を拒否する。
		{DateFrom: "2026-09-01", DateTo: "2026-08-31"},
		// 集計負荷を制限するため366日を超える期間を拒否する。
		{DateFrom: "2025-01-01", DateTo: "2026-01-02"},
	}
	for _, request := range cases {
		repository := &stubSpendingRepository{}
		_, err := NewSpendingService(repository).SpendingSummary(context.Background(), 42, request)
		if !errors.Is(err, ErrInvalidSpendingQuery) {
			t.Fatalf("expected invalid spending query for %#v, got %v", request, err)
		}
		if repository.calledCount != 0 {
			t.Fatalf("expected invalid period not to reach repository, got %d calls", repository.calledCount)
		}
	}
}

// TestSpendingServiceAcceptsMaximumPeriod は、閏年を含む366日間を有効な上限として扱うことを確認する。
// 必要な理由: 負荷制限の境界で正当な1年間の集計を誤って拒否しないため。
func TestSpendingServiceAcceptsMaximumPeriod(t *testing.T) {
	repository := &stubSpendingRepository{summary: SpendingSummary{Currencies: []SpendingCurrency{}}}
	_, err := NewSpendingService(repository).SpendingSummary(context.Background(), 42, SpendingSummaryRequest{
		DateFrom: "2024-01-01",
		DateTo:   "2024-12-31",
	})
	if err != nil {
		t.Fatalf("expected 366-day period to be valid: %v", err)
	}
	if repository.calledCount != 1 {
		t.Fatalf("expected maximum valid period to reach repository, got %d calls", repository.calledCount)
	}
}

// TestSpendingServiceReturnsRepositoryFailure は、集計障害を空データへ変換せず返すことを確認する。
// 必要な理由: DB障害を支出0円と誤表示せず、HTTP境界が安全な500へ変換できるようにするため。
func TestSpendingServiceReturnsRepositoryFailure(t *testing.T) {
	repository := &stubSpendingRepository{err: errors.New("spending query unavailable")}
	if _, err := NewSpendingService(repository).SpendingSummary(context.Background(), 42, SpendingSummaryRequest{DateFrom: "2026-08-01", DateTo: "2026-08-31"}); err == nil {
		t.Fatal("expected repository failure")
	}
}

// TestPostgresRepositorySpendingSummaryQueryFailure は、集計query失敗を呼び出し元へ返すことを確認する。
// 必要な理由: DB障害を空の支出集計と誤認せず、HTTP境界で安全な500へ変換するため。
func TestPostgresRepositorySpendingSummaryQueryFailure(t *testing.T) {
	_, err := NewPostgresRepository(failingQueryer{}).SpendingSummary(context.Background(), 42, spendingSummaryQuery{})
	if err == nil {
		t.Fatal("expected the repository to return its query error")
	}
}
