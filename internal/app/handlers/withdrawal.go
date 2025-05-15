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

type CreateWithdrawalHandler struct {
	withdrawalService service.WithdrawalServiceInterface
}

func NewCreateWithdrawalHandler(withdrawalService service.WithdrawalServiceInterface) CreateWithdrawalHandler {
	return CreateWithdrawalHandler{
		withdrawalService: withdrawalService,
	}
}

func (create CreateWithdrawalHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if contentType := request.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		logger.Log.Infoln("Inappropriate content type passed")
		http.Error(writer, "Only application/json content type is allowed", http.StatusBadRequest)
		return
	}

	defer func(Body io.ReadCloser) {
		innerErr := Body.Close()
		if innerErr != nil {
			logger.Log.Errorf("error closing body: %v", innerErr)
		}
	}(request.Body)
	var requestData models.CreateWithdrawalRequest
	dec := json.NewDecoder(request.Body)
	if err := dec.Decode(&requestData); err != nil {
		logger.Log.Debugf("Couldn't decode the request body: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	intOrderNumber, err := strconv.Atoi(requestData.Order)
	if err != nil {
		logger.Log.Infof("Couldn't convert order number to int: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if !luhn.Valid(intOrderNumber) {
		logger.Log.Infof("Invalid order number: %s", requestData.Order)
		http.Error(writer, "The provided payload does not contain a valid order number", http.StatusUnprocessableEntity)
		return
	}
	if requestData.Amount == 0 {
		logger.Log.Infof("Invalid withdrawal amount for order number: %s", requestData.Order)
		http.Error(writer, "The provided payload does not contain a valid withdrawal amount", http.StatusUnprocessableEntity)
	}
	userID := request.Context().Value(middlewares.UserIDKey).(uint64)
	_, err = create.withdrawalService.Create(request.Context(), requestData.Order, requestData.Amount, userID)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrWithdrawalOrderAlreadyExists):
			logger.Log.Infof("withdrawal order %s already registered", requestData.Order)
			writer.WriteHeader(http.StatusBadRequest)
			return
		case errors.Is(err, repositories.ErrNotEnoughPoints):
			writer.WriteHeader(http.StatusPaymentRequired)
			return
		default:
			logger.Log.Warn("Couldn't register the withdrawal order: ", err)
			http.Error(writer, "Couldn't register the withdrawal", http.StatusInternalServerError)
			return
		}
	}
	writer.WriteHeader(http.StatusOK)
}

type ReadAllWithdrawalsHandler struct {
	withdrawalService service.WithdrawalServiceInterface
}

func NewReadAllWithdrawalsHandler(withdrawalService service.WithdrawalServiceInterface) ReadAllWithdrawalsHandler {
	return ReadAllWithdrawalsHandler{
		withdrawalService: withdrawalService,
	}
}

func (read ReadAllWithdrawalsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(middlewares.UserIDKey).(uint64)
	withdrawals, err := read.withdrawalService.ReadAllByUserID(request.Context(), userID)
	if err != nil {
		http.Error(writer, "Couldn't load withdrawals", http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(writer)
	if err = enc.Encode(withdrawals); err != nil {
		logger.Log.Debugf("Error encoding response: %s", err)
		return
	}
}
