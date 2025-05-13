package repositories

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"slices"
	"time"
)

type Order struct {
	ID        uint64
	UserID    uint64
	Number    string
	Status    string
	CreatedAt time.Time
}

type OrderWithAccrual struct {
	Order
	Accrual sql.NullFloat64
}

const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusProcessed  = "PROCESSED"
	OrderStatusInvalid    = "INVALID"
)

var AllOrderStatuses = []string{OrderStatusNew, OrderStatusProcessing, OrderStatusProcessed, OrderStatusInvalid}
var updateStatusQuery = `UPDATE "order" SET status = $1 WHERE id = $2`

type OrderRepositoryInterface interface {
	Create(ctx context.Context, number string, userID uint64) (Order, error)
	Read(ctx context.Context, number string) (Order, error)
	ReadAllByUserID(ctx context.Context, userID uint64) ([]OrderWithAccrual, error)
	ReadByStatus(ctx context.Context, status string) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, orderID uint64, status string) error
	UpdateOrderAndPasteAccrual(ctx context.Context, order Order, status string, amount float64) error
}

var ErrOrderAlreadyExists = errors.New("order with given number already exists")
var ErrInsertingOrder = errors.New("error registering router")
var ErrOrderNotFound = errors.New("order not found")
var ErrInvalidStatus = errors.New("invalid status passed for order update")
var ErrWrongMethodUsed = errors.New("wrong method used to update order")

type OrderRepository struct {
	pool *sql.DB
}

func NewOrderRepository(pool *sql.DB) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (o OrderRepository) Create(ctx context.Context, number string, userID uint64) (Order, error) {
	insertOrderPreparedStmt, err := o.pool.PrepareContext(
		ctx,
		`INSERT INTO "order" (number, user_id)
				VALUES ($1, $2) 
				RETURNING id, user_id, number, status, created_at`)
	if err != nil {
		logger.Log.Warnf("Error preparing statement for creating order, error %e", err)
		return Order{}, err
	}
	row := insertOrderPreparedStmt.QueryRowContext(ctx, number, userID)
	if row.Err() != nil {
		var pgErr *pgconn.PgError
		if errors.As(row.Err(), &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) {
			logger.Log.Infof("Order %s already registered", number)
			return Order{}, ErrOrderAlreadyExists
		}
		return Order{}, row.Err()
	}
	var ID uint64
	var selectedUserID uint64
	var selectedNumber string
	var status string
	var createdAt time.Time
	err = row.Scan(&ID, &selectedUserID, &selectedNumber, &status, &createdAt)
	if err != nil {
		return Order{}, err
	}
	if row == nil {
		return Order{}, ErrInsertingOrder
	}

	return Order{
		ID:        ID,
		UserID:    userID,
		Number:    number,
		Status:    status,
		CreatedAt: createdAt,
	}, nil
}

func (o OrderRepository) Read(ctx context.Context, number string) (Order, error) {
	selectOrderPreparedStmt, err := o.pool.PrepareContext(
		ctx, `SELECT id, user_id, number, status, created_at FROM "order" WHERE number = $1`)
	if err != nil {
		logger.Log.Warnf("Error preparing query for order, error %e", err)
		return Order{}, err
	}
	row := selectOrderPreparedStmt.QueryRowContext(ctx, number)
	if row.Err() != nil {
		logger.Log.Warnf("Error querying for order number %s, error %e", number, row.Err())
		return Order{}, err
	}
	var ID uint64
	var selectedUserID uint64
	var selectedNumber string
	var status string
	var createdAt time.Time
	err = row.Scan(&ID, &selectedUserID, &selectedNumber, &status, &createdAt)
	if err != nil {
		return Order{}, err
	}
	if row == nil {
		return Order{}, ErrOrderNotFound
	}
	return Order{
		ID:        ID,
		UserID:    selectedUserID,
		Number:    number,
		Status:    status,
		CreatedAt: createdAt,
	}, nil
}

func (o OrderRepository) ReadAllByUserID(ctx context.Context, userID uint64) ([]OrderWithAccrual, error) {
	selectAllOrdersByUserIDPreparedStmt, err := o.pool.PrepareContext(
		ctx,
		`SELECT o.id, o.user_id, o.number, o.status, o.created_at, a.amount 
				FROM "order" o LEFT JOIN "accrual" a on o.id = a.order_id 
				WHERE o.user_id = $1
				ORDER BY o.created_at DESC`)
	if err != nil {
		logger.Log.Warnf("Error preparing statement for quering orders by user %d, err %e", userID, err)
		return nil, err
	}
	rows, err := selectAllOrdersByUserIDPreparedStmt.QueryContext(ctx, userID)
	if err != nil {
		logger.Log.Infof("Error querying orders by user %d, err %e", userID, err)
		return nil, err
	}
	if rows.Err() != nil {
		logger.Log.Infof("Error querying orders by user %d, err %e", userID, err)
		return nil, err
	}
	defer func(rows *sql.Rows) {
		innerErr := rows.Close()
		if innerErr != nil {
			logger.Log.Errorf("error closing rows: %v", innerErr)
		}
	}(rows)
	var orders []OrderWithAccrual
	for rows.Next() {
		order := new(OrderWithAccrual)
		scanErr := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.CreatedAt, &order.Accrual)
		if scanErr != nil {
			logger.Log.Error(scanErr.Error())
			return nil, scanErr
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (o OrderRepository) ReadByStatus(ctx context.Context, status string) ([]Order, error) {
	selectOrdersByStatusPreparedStmt, err := o.pool.PrepareContext(
		ctx,
		`SELECT o.id, o.user_id, o.number, o.status, o.created_at
				FROM "order" o
				WHERE o.status = $1
				ORDER BY o.created_at`)
	if err != nil {
		logger.Log.Warnf("Error preparing statement for quering orders by status err %e", err)
		return nil, err
	}
	rows, err := selectOrdersByStatusPreparedStmt.QueryContext(ctx, status)
	if err != nil {
		logger.Log.Info("Error querying orders by status err %e", err)
		return nil, err
	}
	defer func(rows *sql.Rows) {
		innerErr := rows.Close()
		if innerErr != nil {
			logger.Log.Errorf("error closing rows: %v", innerErr)
		}
	}(rows)
	if rows.Err() != nil {
		if errors.Is(rows.Err(), sql.ErrNoRows) {
			return []Order{}, nil
		}
		logger.Log.Warn("Error querying orders by status err %e", rows.Err())
		return nil, err
	}
	var orders []Order
	for rows.Next() {
		order := new(Order)
		scanErr := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Number,
			&order.Status,
			&order.CreatedAt,
		)
		if scanErr != nil {
			logger.Log.Error(scanErr.Error())
			return nil, scanErr
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

func (o OrderRepository) UpdateOrderStatus(ctx context.Context, orderID uint64, status string) error {
	if !slices.Contains(AllOrderStatuses, status) {
		return ErrInvalidStatus
	}
	if status == OrderStatusProcessed {
		return ErrWrongMethodUsed
	}

	updateStatusPreparedStmt, err := o.pool.PrepareContext(ctx, updateStatusQuery)
	if err != nil {
		logger.Log.Warnf("Error preparing statement for updating status of order %d, err %e", orderID, err)
		return err
	}
	_, err = updateStatusPreparedStmt.ExecContext(ctx, status, orderID)
	if err != nil {
		logger.Log.Infof("Error updating status of order %d, err %e", orderID, err)
		return err
	}
	return nil
}

func (o OrderRepository) UpdateOrderAndPasteAccrual(
	ctx context.Context, order Order, status string, amount float64) error {
	if status != OrderStatusProcessed {
		return ErrWrongMethodUsed
	}

	transaction, txErr := o.pool.BeginTx(ctx, nil)
	if txErr != nil {
		logger.Log.Warnf("Error creating transaction for updating order status, err %e", txErr)
		return txErr
	}

	updateStatusPreparedStmt, err := transaction.PrepareContext(ctx, updateStatusQuery)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error preparing update status statement, err %e", err)
		return err
	}
	_, err = updateStatusPreparedStmt.ExecContext(ctx, status, order.ID)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error executing update status statement, err %e", err)
		return err
	}

	createAccrualPreparedStmt, err := transaction.PrepareContext(
		ctx,
		`INSERT INTO "accrual" (amount, user_id, order_id) VALUES ($1, $2, $3)`)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error preparing insert acctrual statement, err %e", err)
		return err
	}
	_, err = createAccrualPreparedStmt.ExecContext(ctx, amount, order.UserID, order.ID)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error executing update status statement, err %e", err)
		return err
	}

	updateBalancePreparedStmt, err := transaction.PrepareContext(
		ctx,
		`UPDATE "user-balance" SET balance = balance + $1 WHERE user_id = $2`)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error preparing update user balance statement, err %e", err)
		return err
	}
	_, err = updateBalancePreparedStmt.ExecContext(ctx, amount, order.UserID)
	if err != nil {
		txErr = transaction.Rollback()
		if txErr != nil {
			logger.Log.Warnf("Error during transaction rollback, err %e", txErr)
			return txErr
		}
		logger.Log.Warnf("Error executing update user balance statement, err %e", err)
		return err
	}

	txErr = transaction.Commit()
	if txErr != nil {
		logger.Log.Warnf("Error during transaction commit, err %e", txErr)
		return txErr
	}
	return nil
}
