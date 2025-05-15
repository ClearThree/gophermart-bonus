package repositories

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

type Withdrawal struct {
	ID          uint64    `json:"-"`
	UserID      uint64    `json:"-"`
	OrderNumber string    `json:"order"`
	Amount      float64   `json:"sum"`
	CreatedAt   time.Time `json:"processed_at"`
}

type WithdrawalRepositoryInterface interface {
	Create(ctx context.Context, number string, amount float64, userID uint64) (uint64, error)
	ReadAllByUserID(ctx context.Context, userID uint64) ([]Withdrawal, error)
}

var ErrNotEnoughPoints = errors.New("not enough points")
var ErrWithdrawalOrderAlreadyExists = errors.New(" withdrawal order number already exists")

type WithdrawalRepository struct {
	pool *sql.DB
}

func NewWithdrawalRepository(pool *sql.DB) WithdrawalRepositoryInterface {
	return &WithdrawalRepository{pool}
}

func (w WithdrawalRepository) Create(ctx context.Context, number string, amount float64, userID uint64) (uint64, error) {
	transaction, txErr := w.pool.BeginTx(ctx, nil)
	if txErr != nil {
		return 0, txErr
	}

	selectUserBalance, err := transaction.PrepareContext(
		ctx, `SELECT balance FROM "user-balance" WHERE user_id = $1 FOR UPDATE`)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			return 0, txErr
		}
		return 0, err
	}

	row := selectUserBalance.QueryRowContext(ctx, userID)
	if row.Err() != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error preparing select for balance: %v", err)
		return 0, row.Err()
	}

	var balance float64
	err = row.Scan(&balance)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error acquiring balance: %v", err)
		return 0, err
	}

	if amount > balance {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf(
			"error insufficient balance for withdrawal userID %d, withdrawalOrderID %s", userID, number)
		return 0, ErrNotEnoughPoints
	}
	updateUserBalancePreparedStmt, err := transaction.PrepareContext(
		ctx,
		`UPDATE "user-balance" 
			    SET balance = balance - $1, withdrawals_sum = withdrawals_sum + $1 WHERE user_id = $2`)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error preparing update for balance: %v", err)
		return 0, err
	}
	_, err = updateUserBalancePreparedStmt.ExecContext(ctx, amount, userID)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error updating user balance during withdrawal, userID %d", userID)
		return 0, err
	}
	createWithdrawalPreparedStmt, err := transaction.PrepareContext(
		ctx, `INSERT INTO withdrawal (amount, user_id, withdrawal_order_number) VALUES ($1, $2, $3) RETURNING id`)
	if err != nil {
		return 0, err
	}

	row = createWithdrawalPreparedStmt.QueryRowContext(ctx, amount, userID, number)
	if row.Err() != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Fatal("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error creating withdrawal: %v", row.Err())
	}

	var ID uint64
	err = row.Scan(&ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			logger.Log.Infof("Withdrawal order number %s already exists", number)
			return 0, ErrWithdrawalOrderAlreadyExists
		}
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warn("error during transaction rollback")
			return 0, txErr
		}
		logger.Log.Warnf("error creating withdrawal: %v", err)
		return 0, err
	}
	txErr = transaction.Commit()
	if txErr != nil {
		logger.Log.Fatal("error during transaction commit")
		return 0, txErr
	}
	return ID, nil

}

func (w WithdrawalRepository) ReadAllByUserID(ctx context.Context, userID uint64) ([]Withdrawal, error) {
	selectAllWithdrawalsStmt, err := w.pool.PrepareContext(
		ctx,
		"SELECT id, amount, user_id, created_at, withdrawal_order_number FROM withdrawal WHERE user_id = $1")
	if err != nil {
		logger.Log.Error("error during prepare withdrawals select")
		return nil, err
	}
	rows, err := selectAllWithdrawalsStmt.QueryContext(ctx, userID)
	if err != nil {
		logger.Log.Error("error during withdrawals selection")
		return nil, err
	}
	if rows.Err() != nil {
		logger.Log.Fatal("error during withdrawals selection")
		return nil, rows.Err()
	}
	defer func(rows *sql.Rows) {
		innerErr := rows.Close()
		if innerErr != nil {
			logger.Log.Error("error closing rows: %v", innerErr)
		}
	}(rows)
	var withdrawals []Withdrawal
	for rows.Next() {
		withdrawal := new(Withdrawal)
		scanErr := rows.Scan(&withdrawal.ID, &withdrawal.Amount, &withdrawal.UserID, &withdrawal.CreatedAt, &withdrawal.OrderNumber)
		if scanErr != nil {
			logger.Log.Error(scanErr.Error())
			return nil, scanErr
		}
		withdrawals = append(withdrawals, *withdrawal)
	}
	return withdrawals, nil
}
