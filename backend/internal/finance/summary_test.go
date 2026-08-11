package finance

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type stubRepository struct {
	summary    Summary
	seenUserID int64
}

func (s *stubRepository) Summary(_ context.Context, userID int64) (Summary, error) {
	s.seenUserID = userID
	return s.summary, nil
}

// TestServiceSummary は、認証済みuser IDを変更せずrepositoryへ渡すことを確認する。
// 必要な理由: use case境界で所有者条件が欠落・置換されると、別ユーザーの集計につながるため。
func TestServiceSummary(t *testing.T) {
	repository := &stubRepository{summary: Summary{AccountCount: 2}}
	service := NewService(repository)

	summary, err := service.Summary(context.Background(), 42)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	if repository.seenUserID != 42 {
		t.Fatalf("expected user ID 42, got %d", repository.seenUserID)
	}
	if summary.AccountCount != 2 {
		t.Fatalf("expected repository summary, got %#v", summary)
	}
}

type failingQueryer struct{}

func (failingQueryer) QueryContext(_ context.Context, _ string, _ ...any) (*sql.Rows, error) {
	return nil, errors.New("query unavailable")
}

// TestPostgresRepositorySummaryQueryFailure は、集計query失敗を呼び出し元へ返すことを確認する。
// 必要な理由: DB障害を空データとして扱うと、UIが障害を正常な0件と誤表示するため。
func TestPostgresRepositorySummaryQueryFailure(t *testing.T) {
	_, err := NewPostgresRepository(failingQueryer{}).Summary(context.Background(), 1)
	if err == nil {
		t.Fatal("expected the repository to return its query error")
	}
}
