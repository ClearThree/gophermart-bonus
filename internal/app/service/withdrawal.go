package service

import (
	"context"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
)

type WithdrawalServiceInterface interface {
	Create(ctx context.Context, number string, amount float64, userID uint64) (uint64, error)
	ReadAllByUserID(ctx context.Context, userID uint64) ([]repositories.Withdrawal, error)
}

type WithdrawalService struct {
	withdrawalRepository repositories.WithdrawalRepositoryInterface
}

func NewWithdrawalService(withdrawalRepository repositories.WithdrawalRepositoryInterface) *WithdrawalService {
	return &WithdrawalService{
		withdrawalRepository: withdrawalRepository,
	}
}

func (w WithdrawalService) Create(ctx context.Context, number string, amount float64, userID uint64) (uint64, error) {
	createdWithdrawalID, err := w.withdrawalRepository.Create(ctx, number, amount, userID)
	if err != nil {
		return 0, err
	}
	return createdWithdrawalID, nil
}

func (w WithdrawalService) ReadAllByUserID(ctx context.Context, userID uint64) ([]repositories.Withdrawal, error) {
	existingWithdrawals, err := w.withdrawalRepository.ReadAllByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return existingWithdrawals, nil
}
