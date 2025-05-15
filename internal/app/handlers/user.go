package handlers

import (
	"encoding/json"
	"errors"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/ClearThree/gophermart-bonus/internal/app/middlewares"
	"github.com/ClearThree/gophermart-bonus/internal/app/models"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
	"github.com/ClearThree/gophermart-bonus/internal/app/service"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type RegisterHandler struct {
	userService service.UserServiceInterface
}

func NewRegisterHandler(service service.UserServiceInterface) *RegisterHandler {
	return &RegisterHandler{userService: service}
}

func (register RegisterHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if contentType := request.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		logger.Log.Infoln("Inappropriate content type passed")
		http.Error(writer, "Only application/json content type is allowed", http.StatusBadRequest)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Log.Errorf("error closing body: %v", err)
		}
	}(request.Body)
	var requestData models.LoginPasswordRequest
	dec := json.NewDecoder(request.Body)
	if err := dec.Decode(&requestData); err != nil {
		logger.Log.Debugf("Couldn't decode the request body: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if requestData.Login == "" || requestData.Password == "" {
		http.Error(writer, "Both login and password should be passed", http.StatusBadRequest)
		return
	}
	id, err := register.userService.Register(request.Context(), requestData.Login, requestData.Password)
	if err != nil {
		if errors.Is(err, repositories.ErrLoginAlreadyTaken) {
			http.Error(writer, "Passed login already exists", http.StatusConflict)
			return
		}
		logger.Log.Warnf("Failed to register user %v", err)
		http.Error(writer, "Couldn't register user, something went wrong", http.StatusInternalServerError)
		return
	}
	writer.Header().Add(string(middlewares.UserIDKey), strconv.FormatUint(id, 10))
	writer.WriteHeader(http.StatusOK)
}

type LoginHandler struct {
	userService service.UserServiceInterface
}

func NewLoginHandler(service service.UserServiceInterface) *LoginHandler {
	return &LoginHandler{userService: service}
}

func (login LoginHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if contentType := request.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		logger.Log.Infoln("Inappropriate content type passed")
		http.Error(writer, "Only application/json content type is allowed", http.StatusBadRequest)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Log.Errorf("Failed to close body: %v", err)
		}
	}(request.Body)
	var requestData models.LoginPasswordRequest
	dec := json.NewDecoder(request.Body)
	if err := dec.Decode(&requestData); err != nil {
		logger.Log.Infof("Couldn't decode the request body: %s", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if requestData.Login == "" || requestData.Password == "" {
		logger.Log.Infof("Empty login and/or password passed")
		http.Error(writer, "Both login and password should be passed", http.StatusBadRequest)
		return
	}
	id, err := login.userService.Authenticate(request.Context(), requestData.Login, requestData.Password)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrUserNotFound):
			logger.Log.Warnf("Failed to authenticate user %v", err)
			http.Error(writer, "No user found with the given login", http.StatusUnauthorized)
			return
		case errors.Is(err, service.ErrPasswordIsIncorrect):
			logger.Log.Warnf("Failed to authenticate user %v", err)
			http.Error(writer, "Password is incorrect for the given user", http.StatusUnauthorized)
			return
		default:
			logger.Log.Warnf("Failed to authenticate user %v", err)
			http.Error(writer, "Couldn't authenticate user, something went wrong", http.StatusInternalServerError)
			return
		}
	}
	writer.Header().Add(string(middlewares.UserIDKey), strconv.FormatUint(id, 10))
	writer.WriteHeader(http.StatusOK)
}

type UserBalancesHandler struct {
	userService service.UserServiceInterface
}

func NewUserBalancesHandler(service service.UserServiceInterface) *UserBalancesHandler {
	return &UserBalancesHandler{userService: service}
}

func (balances UserBalancesHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Log.Errorf("Failed to close body: %v", err)
		}
	}(request.Body)
	userID := request.Context().Value(middlewares.UserIDKey).(uint64)
	current, withdrawn, err := balances.userService.GetBalances(request.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			http.Error(writer, "No user found with the given userID", http.StatusInternalServerError)
			return
		}
		logger.Log.Warnf("Failed to get user balances %v", err)
		http.Error(writer, "Couldn't get user balances", http.StatusInternalServerError)
		return
	}
	responseData := models.GetBalancesResponse{
		Current:   current,
		Withdrawn: withdrawn,
	}
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(writer)
	if err = enc.Encode(responseData); err != nil {
		logger.Log.Debugf("Error encoding response: %s", err)
		return
	}
}
