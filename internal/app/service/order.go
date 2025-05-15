package service

import (
	"context"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
	"time"
)

type OrderServiceInterface interface {
	Create(ctx context.Context, number string, userID uint64) (uint64, error)
	ReadAllByUserID(ctx context.Context, userID uint64) ([]repositories.OrderWithAccrual, error)
	GetOrdersForProcessing(ctx context.Context) ([]repositories.Order, error)
	UpdateOrderStatus(ctx context.Context, order repositories.Order) error
}

var ErrOrderAlreadyRegisteredByCurrentUser = errors.New("order already registered by current user")

type OrderService struct {
	orderRepository   repositories.OrderRepositoryInterface
	accrualRepository repositories.AccrualRepositoryInterface
}

func NewOrderService(
	orderRepository repositories.OrderRepositoryInterface,
	accrualRepository repositories.AccrualRepositoryInterface) *OrderService {
	return &OrderService{
		orderRepository:   orderRepository,
		accrualRepository: accrualRepository,
	}
}

func (o OrderService) Create(ctx context.Context, number string, userID uint64) (uint64, error) {
	order, err := o.orderRepository.Create(ctx, number, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrderAlreadyExists) {
			existingOrder, innerErr := o.orderRepository.Read(ctx, number)
			if innerErr != nil {
				return 0, innerErr
			}
			if existingOrder.UserID != userID {
				return 0, err
			} else {
				return 0, ErrOrderAlreadyRegisteredByCurrentUser
			}
		}
		return 0, err
	}
	return order.ID, nil
}

func (o OrderService) ReadAllByUserID(ctx context.Context, userID uint64) ([]repositories.OrderWithAccrual, error) {
	orders, err := o.orderRepository.ReadAllByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (o OrderService) GetOrdersForProcessing(ctx context.Context) ([]repositories.Order, error) {
	orders, err := o.orderRepository.ReadByStatus(ctx, "NEW")
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func (o OrderService) UpdateOrderStatus(ctx context.Context, order repositories.Order) error {
	orderState, err := o.accrualRepository.GetOrder(order.Number)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrOrderNotRegistered):
			innerErr := o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusInvalid)
			if innerErr != nil {
				logger.Log.Warnf("Error updating order with number %s to status %s", order.Number, order.Status)
				return innerErr
			}
			return nil
		default:
			logger.Log.Warnf("Error getting order from accrual system, orderID %d, passing for now", order.ID)
			err = o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusNew)
			if err != nil {
				logger.Log.Warnf("Failed to update order with NEW status: %v", err)
				return err
			}
			return nil
		}
	}

	switch orderState.Status {
	case repositories.ExternalOrderStatusRegistered:
	case repositories.ExternalOrderStatusProcessing:
		logger.Log.Warnf("Order is still in processing, orderID %d, passing for now", order.ID)
		err = o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusNew)
		if err != nil {
			logger.Log.Warnf("Failed to update order with NEW status: %v", err)
			return err
		}
		return nil
	case repositories.ExternalOrderStatusProcessed:
		err = o.orderRepository.UpdateOrderAndPasteAccrual(
			ctx, order, repositories.OrderStatusProcessed, orderState.Accrual)
		if err != nil {
			logger.Log.Warnf("Failed to update order %s with PROCESSED status: %v", order.Number, err)
			innerErr := o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusNew)
			if innerErr != nil {
				logger.Log.Error("Error returning order to NEW: %v", innerErr)
				return innerErr
			}
			return err
		}
		return nil
	case repositories.ExternalOrderStatusInvalid:
		err = o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusInvalid)
		if err != nil {
			logger.Log.Warnf("Failed to update order %s with INVALID status: %v", order.Number, err)
			innerErr := o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusNew)
			if innerErr != nil {
				logger.Log.Error("Error returning order to NEW: %v", innerErr)
				return innerErr
			}
			return err
		}
		return nil
	default:
		logger.Log.Warnf("Order %s is in unknown status: %s", order.Number, orderState.Status)
		err = o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusNew)
		if err != nil {
			logger.Log.Warnf("Failed to update order %s with NEW status: %v", order.Number, err)
			return err
		}
	}
	return nil
}

func (o OrderService) WorkerLoop(ctx context.Context) error {
	ordersChannel := make(chan repositories.Order, config.Settings.DefaultChannelsBufferSize)
	errorsChannel := make(chan error)
	for i := 0; i < int(config.Settings.WorkersNumber); i++ {
		go func() {
			err := o.Worker(ctx, ordersChannel, errorsChannel)
			if err != nil {
				logger.Log.Warnf("Worker Exited with error: %v", err)
			}
		}()
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			orders, err := o.GetOrdersForProcessing(ctx)
			if err != nil {
				logger.Log.Warnf("Failed to get orders for processing: %v", err)
				return err
			}
			for _, order := range orders {
				err = o.orderRepository.UpdateOrderStatus(ctx, order.ID, repositories.OrderStatusProcessing)
				if err != nil {
					logger.Log.Warnf("Failed to update order with PROCESSING status: %v", err)
					return err
				}
				ordersChannel <- order
			}
			time.Sleep(config.Settings.OrderStatusCheckPeriod)
		case err := <-errorsChannel:
			logger.Log.Warnf("Worker reported an error: %v", err)
		}
	}
}

func (o OrderService) Worker(
	ctx context.Context, ordersChannel <-chan repositories.Order, errorsChannel chan<- error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case order := <-ordersChannel:
			logger.Log.Debugf("Worker received order: %v", order)
			err := o.UpdateOrderStatus(ctx, order)
			if err != nil {
				errorsChannel <- err
			}
		}
	}
}
