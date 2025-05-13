package server

import (
	"context"
	"database/sql"
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"github.com/ClearThree/gophermart-bonus/internal/app/handlers"
	"github.com/ClearThree/gophermart-bonus/internal/app/logger"
	"github.com/ClearThree/gophermart-bonus/internal/app/middlewares"
	"github.com/ClearThree/gophermart-bonus/internal/app/repositories"
	"github.com/ClearThree/gophermart-bonus/internal/app/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose"
	"net/http"
	"os"
	"time"
)

var Pool *sql.DB

func GophermartBonusRouter(pool *sql.DB) chi.Router {
	orderService := service.NewOrderService(
		repositories.NewOrderRepository(pool),
		repositories.NewAccrualRepository(&config.Settings))
	userService := service.NewUserService(repositories.NewUserRepository(pool))
	withdrawalService := service.NewWithdrawalService(repositories.NewWithdrawalRepository(pool))

	var registerHandler = handlers.NewRegisterHandler(userService)
	var loginHandler = handlers.NewLoginHandler(userService)
	var userBalancesHandler = handlers.NewUserBalancesHandler(userService)
	var registerOrderHandler = handlers.NewRegisterOrderHandler(orderService)
	var readAllOrdersHandler = handlers.NewReadAllOrdersHandler(orderService)
	var createWithdrawalHandler = handlers.NewCreateWithdrawalHandler(withdrawalService)
	var readAllWithdrawalsHandler = handlers.NewReadAllWithdrawalsHandler(withdrawalService)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middlewares.RequestLogger)
	router.Use(middlewares.ValidationMiddleware)
	router.Use(middlewares.GzipMiddleware)
	router.Use(middleware.Recoverer)

	router.Route("/api/user", func(r chi.Router) {

		noAuthGroup := r.Group(nil)
		noAuthGroup.Use(middlewares.SetAuthMiddleware)
		noAuthGroup.Post("/register", registerHandler.ServeHTTP)
		noAuthGroup.Post("/login", loginHandler.ServeHTTP)

		authGroup := r.Group(nil)
		authGroup.Use(middlewares.AuthMiddleware)
		authGroup.Get("/balance", userBalancesHandler.ServeHTTP)
		authGroup.Post("/orders", registerOrderHandler.ServeHTTP)
		authGroup.Get("/orders", readAllOrdersHandler.ServeHTTP)
		authGroup.Post("/balance/withdraw", createWithdrawalHandler.ServeHTTP)
		authGroup.Get("/withdrawals", readAllWithdrawalsHandler.ServeHTTP)
	})
	go func() {
		err := orderService.WorkerLoop(context.Background())
		if err != nil {
			logger.Log.Errorf("Error in orderService.WorkerLoop: %v", err)
		}
	}()
	return router
}

func Run(addr string) error {
	logger.Log.Infof("Initiating server at %s", addr)
	if config.Settings.DatabaseURI == "" {
		logger.Log.Fatal("no Database URI provided")
		os.Exit(1)
	}

	var err error
	Pool, err = sql.Open("pgx", config.Settings.DatabaseURI)
	if err != nil {
		return err
	}
	defer func(Pool *sql.DB) {
		innerErr := Pool.Close()
		if innerErr != nil {
			logger.Log.Errorf("error closing pool: %v", innerErr)
		}
	}(Pool)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err = Pool.PingContext(ctx); err != nil {
		return err
	}

	err = migrateDB(Pool)
	if err != nil {
		return err
	}
	logger.Log.Info("Server initiation completed, starting to serve")

	return http.ListenAndServe(addr, GophermartBonusRouter(Pool))
}

func migrateDB(pool *sql.DB) error {
	return goose.Up(pool, "migrations")
}
