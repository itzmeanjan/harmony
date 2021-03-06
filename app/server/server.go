package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Start - Life cycle definition of http server
func Start(ctx context.Context, res *data.Resource) {

	router := echo.New()

	router.Use(middleware.LoggerWithConfig(
		middleware.LoggerConfig{
			Format: "${time_rfc3339} | ${method} | ${uri} | ${status} | ${remote_ip} | ${latency_human}\n",
		}))

	v1 := router.Group("/v1")

	{

		v1.GET("/stat", func(c echo.Context) error {

			return c.JSON(http.StatusOK, &data.Stat{
				PendingPoolSize: res.Pool.PendingPoolLength(),
				QueuedPoolSize:  res.Pool.QueuedPoolLength(),
			})

		})

	}

	if err := router.Start(fmt.Sprintf(":%d", config.GetPortNumber())); err != nil {

		log.Printf("[❌] Failed to start http server : %s\n", err.Error())

	}

}
