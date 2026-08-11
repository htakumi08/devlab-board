package finance

import (
	"context"
	"errors"
	"testing"
)

type stubAccountRepository struct {
	accounts            []Account
	detail              AccountDetail
	err                 error
	seenListUserID      int64
	seenDetailUserID    int64
	seenPublicAccountID string
}

func (s *stubAccountRepository) Accounts(_ context.Context, userID int64) ([]Account, error) {
	s.seenListUserID = userID
	return s.accounts, s.err
}

func (s *stubAccountRepository) Account(_ context.Context, userID int64, publicAccountID string) (AccountDetail, error) {
	s.seenDetailUserID = userID
	s.seenPublicAccountID = publicAccountID
	return s.detail, s.err
}

// TestAccountServicePreservesOwnershipBoundary は、一覧・詳細で認証user IDと公開口座IDを変更しないことを確認する。
// 必要な理由: use case境界で所有者条件が欠落すると、他ユーザーのFinance口座取得につながるため。
func TestAccountServicePreservesOwnershipBoundary(t *testing.T) {
	repository := &stubAccountRepository{
		accounts: []Account{{ID: "00000000-0000-4000-8000-000000000001"}},
		detail:   AccountDetail{Account: Account{ID: "00000000-0000-4000-8000-000000000001"}},
	}
	service := NewAccountService(repository)

	accounts, err := service.Accounts(context.Background(), 42)
	if err != nil || len(accounts) != 1 || repository.seenListUserID != 42 {
		t.Fatalf("expected owner-scoped accounts, got accounts=%#v user=%d err=%v", accounts, repository.seenListUserID, err)
	}
	detail, err := service.Account(context.Background(), 42, repository.detail.ID)
	if err != nil || detail.ID != repository.detail.ID || repository.seenDetailUserID != 42 || repository.seenPublicAccountID != repository.detail.ID {
		t.Fatalf("expected owner-scoped detail, got detail=%#v user=%d account=%q err=%v", detail, repository.seenDetailUserID, repository.seenPublicAccountID, err)
	}
}

// TestAccountServiceHidesInvalidPublicID は、不正UUIDをrepositoryへ渡さずnot foundにすることを確認する。
// 必要な理由: PostgreSQL UUID cast errorを500にせず、未存在・他所有者と同じ外部契約を守るため。
func TestAccountServiceHidesInvalidPublicID(t *testing.T) {
	repository := &stubAccountRepository{}
	_, err := NewAccountService(repository).Account(context.Background(), 42, "not-a-uuid")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected account not found, got %v", err)
	}
	if repository.seenDetailUserID != 0 || repository.seenPublicAccountID != "" {
		t.Fatalf("expected invalid UUID not to reach repository, got user=%d account=%q", repository.seenDetailUserID, repository.seenPublicAccountID)
	}
}
