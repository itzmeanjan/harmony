package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/itzmeanjan/harmony/app/config"
	"github.com/itzmeanjan/harmony/app/data"
	"github.com/itzmeanjan/harmony/app/graph"
	"github.com/itzmeanjan/harmony/app/graph/generated"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Start - Life cycle definition of http server
func Start(ctx context.Context, res *data.Resource) {

	router := echo.New()

	router.Use(middleware.LoggerWithConfig(
		middleware.LoggerConfig{
			Format: "${time_rfc3339} [📩] ${method} | ${uri} | ${status} | ${remote_ip} | ${latency_human}\n",
		}))

	router.Use(middleware.CORSWithConfig(
		middleware.CORSConfig{
			Skipper:      middleware.DefaultSkipper,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPost},
		}))

	v1 := router.Group("/v1")

	graphql := handler.NewDefaultServer(generated.NewExecutableSchema(
		generated.Config{
			Resolvers: &graph.Resolver{},
		}))

	// -- Allowed underlying network transports
	// using which clients can talk to server
	//
	// 👇 to be used for answering queries
	graphql.AddTransport(transport.POST{})
	// 👇 to be used for subscription
	graphql.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	})
	// -- Ends here

	if graphql == nil {

		log.Printf("[❌] Failed to get graphql request handler\n")
		return

	}

	{

		v1.GET("/stat", func(c echo.Context) error {

			return c.JSON(http.StatusOK, &data.Stat{
				PendingPoolSize: res.Pool.PendingPoolLength(),
				QueuedPoolSize:  res.Pool.QueuedPoolLength(),
				Uptime:          time.Now().UTC().Sub(res.StartedAt).String(),
				NetworkID:       res.NetworkID,
			})

		})

		v1.POST("/graphql", func(c echo.Context) error {

			graphql.ServeHTTP(c.Response().Writer, c.Request())
			return nil

		})

		v1.GET("/graphql-playground", func(c echo.Context) error {

			gpg := playground.Handler("harmony", "/v1/graphql")

			if gpg == nil {

				return c.JSON(http.StatusInternalServerError, &data.Msg{
					Message: "Failed to start GraphQL playground",
				})

			}

			gpg.ServeHTTP(c.Response().Writer, c.Request())
			return nil

		})

	}

	if err := router.Start(fmt.Sprintf(":%d", config.GetPortNumber())); err != nil {

		log.Printf("[❌] Failed to start http server : %s\n", err.Error())

	}

}
