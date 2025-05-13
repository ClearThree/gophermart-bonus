package main

import (
	"fmt"
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"github.com/ClearThree/gophermart-bonus/internal/app/server"
	"github.com/caarlos0/env/v6"
	"log"
)

func main() {
	config.ParseFlags()
	err := env.Parse(&config.Settings)
	if err != nil {
		fmt.Println("parsing env variables was not successful: ", err)
	}
	config.Settings.Sanitize()
	if err = server.Run(config.Settings.Address); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
