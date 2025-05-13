package handlers

import (
	"encoding/json"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/ClearThree/gophermart-bonus/internal/app/middlewares"
	"github.com/ClearThree/gophermart-bonus/internal/app/models"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
	"github.com/ClearThree/gophermart-bonus/internal/app/service"
	"github.com/theplant/luhn"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type RegisterOrderHandler struct {
	orderService service.OrderServiceInterface
}

func NewRegisterOrderHandler(service service.OrderServiceInterface) *RegisterOrderHandler {
	return &RegisterOrderHandler{orderService: service}
}

func (register RegisterOrderHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if contentType := request.Header.Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		logger.Log.Infoln("Inappropriate content type passed")
		http.Error(writer, "Only text/plain content type is allowed", http.StatusBadRequest)
		return
	}

	payload, err := io.ReadAll(request.Body)
	if err != nil {
		logger.Log.Warn("Couldn't read the request body")
		http.Error(writer, "Couldn't read the request body", http.StatusBadRequest)
		return
	}
	if len(payload) == 0 {
		logger.Log.Warn("Couldn't read the request body")
		http.Error(writer, "Please provide an order number", http.StatusBadRequest)
		return
	}
	orderNumber := string(payload)
	intOrderNumber, err := strconv.Atoi(orderNumber)
	if err != nil {
		logger.Log.Warn("Couldn't parse the order number: not a number")
		http.Error(writer, "The provided payload is not a valid order number", http.StatusUnprocessableEntity)
	}
	if !luhn.Valid(intOrderNumber) {
		logger.Log.Warnf("Invalid order number: %s", orderNumber)
		http.Error(writer, "The provided payload is not a valid order number", http.StatusUnprocessableEntity)
		return
	}
	userID := request.Context().Value(middlewares.UserIDKey).(uint64)
	ID, err := register.orderService.Create(request.Context(), orderNumber, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOrderAlreadyRegisteredByCurrentUser):
			logger.Log.Infof("order %s with id %d is already registered", orderNumber, ID)
			writer.WriteHeader(http.StatusOK)
			return
		case errors.Is(err, repositories.ErrOrderAlreadyExists):
			writer.WriteHeader(http.StatusConflict)
			return
		default:
			logger.Log.Warnf("Couldn't register the order, err: %e", err)
			http.Error(writer, "Couldn't register the order", http.StatusInternalServerError)
			return
		}
	}
	writer.WriteHeader(http.StatusAccepted)
}

type ReadAllOrdersHandler struct {
	orderService service.OrderServiceInterface
}

func NewReadAllOrdersHandler(service service.OrderServiceInterface) *ReadAllOrdersHandler {
	return &ReadAllOrdersHandler{orderService: service}
}

func (read ReadAllOrdersHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(middlewares.UserIDKey).(uint64)
	orders, err := read.orderService.ReadAllByUserID(request.Context(), userID)
	if err != nil {
		http.Error(writer, "Couldn't load orders", http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	responseData := make([]models.OrdersResponse, len(orders))
	for index, order := range orders {
		responseData[index] = models.OrdersResponse{
			Number:    order.Number,
			Status:    order.Status,
			CreatedAt: order.CreatedAt,
		}
		if order.Accrual.Valid {
			responseData[index].Accrual = order.Accrual.Float64
		}
	}
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(writer)
	if err = enc.Encode(responseData); err != nil {
		logger.Log.Debugf("Error encoding response: %s", err)
		return
	}
}
