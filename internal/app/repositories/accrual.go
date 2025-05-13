package repositories

import (
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"time"
)

type ExternalOrder struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual"`
}

type AccrualRepositoryInterface interface {
	GetOrder(number string) (ExternalOrder, error)
}

const (
	ExternalOrderStatusRegistered = "REGISTERED"
	ExternalOrderStatusProcessing = "PROCESSING"
	ExternalOrderStatusProcessed  = "PROCESSED"
	ExternalOrderStatusInvalid    = "INVALID"
)

var ErrTooManyRequests = errors.New("too many requests")
var ErrOrderNotRegistered = errors.New("order not registered in accrual system")
var ErrExternalAccrualServiceNotAvailable = errors.New("accrual system not available")
var ErrUnexpectedBehaviour = errors.New("accrual system acting unexpectedly")

type AccrualRepository struct {
	config     *config.Config
	client     *resty.Client
	retryAfter time.Time
}

func NewAccrualRepository(config *config.Config) AccrualRepository {
	return AccrualRepository{config: config, client: resty.New()}
}

func (a AccrualRepository) CanDoRequest() bool {
	if a.retryAfter.IsZero() {
		return true
	}
	if a.retryAfter.Before(time.Now()) {
		return true
	}
	return false
}

func (a AccrualRepository) GetSleepDuration() time.Duration {
	if a.retryAfter.IsZero() {
		return 0
	}
	return time.Duration(time.Until(a.retryAfter).Seconds())
}

func (a AccrualRepository) GetOrder(number string) (ExternalOrder, error) {
	if !a.CanDoRequest() {
		time.Sleep(a.GetSleepDuration())
	}
	url := a.config.AccrualSystemAddress + "api/orders/" + number
	order := ExternalOrder{}
	response, err := a.client.R().SetResult(&order).Get(url)
	if err != nil {
		logger.Log.Warn("Error requesting accrual system", zap.String("url", url), zap.Error(err))
		return ExternalOrder{}, err
	}
	if response == nil {
		logger.Log.Warn("accrual service returned nil response")
		return ExternalOrder{}, err
	}
	switch response.StatusCode() {
	case http.StatusTooManyRequests:
		retryAfterHeaderValue, innerErr := strconv.Atoi(response.Header()["Retry-After"][0])
		if innerErr != nil {
			logger.Log.Warnf("Could not parse Retry-After header: %s", innerErr)
			return ExternalOrder{}, innerErr
		}
		logger.Log.Infof("Accrual system reported too many requests, retry after %d", retryAfterHeaderValue)
		a.retryAfter = time.Now().Add(time.Duration(retryAfterHeaderValue))
		return ExternalOrder{}, ErrTooManyRequests
	case http.StatusNoContent:
		logger.Log.Infof("No order registered with number %s", number)
		return ExternalOrder{}, ErrOrderNotRegistered
	case http.StatusInternalServerError:
		logger.Log.Warn("accrual service returned internal server error")
		return ExternalOrder{}, ErrExternalAccrualServiceNotAvailable
	case http.StatusOK:
		return order, nil
	default:
		return ExternalOrder{}, ErrUnexpectedBehaviour
	}
}
