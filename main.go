package main

import (
	"context"
	"log"

	"github.com/adalundhe/micron"
	"github.com/adalundhe/micron/config"
	"github.com/adalundhe/micron/routes"
	"github.com/gin-gonic/gin"
)

func main() {
	
	app, err := micron.Create(&micron.App{
		Server: micron.SeverOptions{
			Name: "Conveyor",
			Description: "A service for scheduling and orchestrating file transfers",
			Version: "v1",
			Port: 5051,
			TLSPort: 5443,
			HealthCheckPort: 5052,
		},
		Build: func(ctx context.Context, router *routes.Router, cfg *config.Config) (*routes.Router, error) {

			router.AddRoute("/test", "GET", routes.RouteConfig{
				Endpoint: func(ctx *gin.Context) (string, error) {
					return "OK", nil
				},
				StatusCode: 200,
			})
		
			return router, nil
		},
	})

	if err != nil {
		log.Fatalf("Encountered error creating Conveyor instance: %s", err.Error())
	}

	if err := app.Run(func() error {
		return nil
	}); err != nil {
		log.Fatalf("Encountered error running Conveyor: %s", err.Error())
	}

}