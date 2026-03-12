package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shengli/prism/sdk"
	"github.com/shengli/prism/sdk/middleware"
)

// This example demonstrates a simple microservice architecture with
// automatic tracing using the Prism SDK.
//
// Architecture:
//   API Gateway (:8081) → Order Service (:8082) → User Service (:8083)
//
// Usage:
//   1. Start infrastructure:  cd deploy && docker compose up -d
//   2. Run this example:      go run ./examples/microservices/
//   3. Send a request:        curl http://localhost:8081/api/orders/123
//   4. View traces:           curl http://localhost:28080/api/v1/traces

func main() {
	// Create tracers for each service
	gatewayTracer := sdk.NewTracer("api-gateway",
		sdk.WithCollector("localhost:24317"),
		sdk.WithSampler(sdk.AlwaysSampler{}),
	)
	defer gatewayTracer.Shutdown()

	orderTracer := sdk.NewTracer("order-service",
		sdk.WithCollector("localhost:24317"),
		sdk.WithSampler(sdk.AlwaysSampler{}),
	)
	defer orderTracer.Shutdown()

	userTracer := sdk.NewTracer("user-service",
		sdk.WithCollector("localhost:24317"),
		sdk.WithSampler(sdk.AlwaysSampler{}),
	)
	defer userTracer.Shutdown()

	// Traced HTTP client for inter-service calls
	orderClient := &http.Client{
		Transport: &middleware.TracedTransport{
			Tracer:  gatewayTracer,
			Wrapped: http.DefaultTransport,
		},
	}
	userClient := &http.Client{
		Transport: &middleware.TracedTransport{
			Tracer:  orderTracer,
			Wrapped: http.DefaultTransport,
		},
	}

	// --- User Service (port 8083) ---
	userMux := http.NewServeMux()
	userMux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// Simulate DB query
		_, span := userTracer.StartSpan(ctx, "sql.Query",
			sdk.WithKind(sdk.KindClient),
			sdk.WithTag("db.type", "postgresql"),
			sdk.WithTag("db.statement", "SELECT * FROM users WHERE id = $1"),
		)
		time.Sleep(10 * time.Millisecond)
		userTracer.FinishSpan(ctx, span)

		json.NewEncoder(w).Encode(map[string]any{
			"id":   "123",
			"name": "Alice",
		})
	})
	userHandler := middleware.HTTPServerMiddleware(userTracer)(userMux)

	// --- Order Service (port 8082) ---
	orderMux := http.NewServeMux()
	orderMux.HandleFunc("/api/orders/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Simulate Redis cache lookup
		_, redisSpan := orderTracer.StartSpan(ctx, "redis.GET",
			sdk.WithKind(sdk.KindClient),
			sdk.WithTag("db.type", "redis"),
			sdk.WithTag("peer.service", "localhost:6379"),
		)
		time.Sleep(2 * time.Millisecond)
		orderTracer.FinishSpan(ctx, redisSpan)

		// Call user service
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:8083/api/users/123", nil)
		resp, err := userClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		resp.Body.Close()

		json.NewEncoder(w).Encode(map[string]any{
			"order_id": "order-456",
			"user_id":  "123",
			"amount":   99.99,
		})
	})
	orderHandler := middleware.HTTPServerMiddleware(orderTracer)(orderMux)

	// --- API Gateway (port 8081) ---
	gwMux := http.NewServeMux()
	gwMux.HandleFunc("/api/orders/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Validate request
		_, valSpan := gatewayTracer.StartSpan(ctx, "validateRequest")
		time.Sleep(1 * time.Millisecond)
		gatewayTracer.FinishSpan(ctx, valSpan)

		// Call order service
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:8082"+r.URL.Path, nil)
		resp, err := orderClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer resp.Body.Close()

		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	})
	gwHandler := middleware.HTTPServerMiddleware(gatewayTracer)(gwMux)

	// Start all services
	go func() {
		log.Println("User Service starting on :8083")
		http.ListenAndServe(":8083", userHandler)
	}()
	go func() {
		log.Println("Order Service starting on :8082")
		http.ListenAndServe(":8082", orderHandler)
	}()

	time.Sleep(100 * time.Millisecond) // Let services start

	fmt.Println("=== Prism Microservices Example ===")
	fmt.Println("Services:")
	fmt.Println("  API Gateway:    http://localhost:8081")
	fmt.Println("  Order Service:  http://localhost:8082")
	fmt.Println("  User Service:   http://localhost:8083")
	fmt.Println("")
	fmt.Println("Try:")
	fmt.Println("  curl http://localhost:8081/api/orders/123")
	fmt.Println("")
	fmt.Println("Then view traces:")
	fmt.Println("  curl http://localhost:28080/api/v1/traces | jq")

	// Generate some sample traffic
	go func() {
		time.Sleep(500 * time.Millisecond)
		for i := 0; i < 5; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), "GET",
				fmt.Sprintf("http://localhost:8081/api/orders/%d", i+1), nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("request %d failed: %v", i+1, err)
				continue
			}
			resp.Body.Close()
			log.Printf("request %d: status %d", i+1, resp.StatusCode)
			time.Sleep(200 * time.Millisecond)
		}
		log.Println("Sample traffic generated!")
	}()

	log.Println("API Gateway starting on :8081")
	if err := http.ListenAndServe(":8081", gwHandler); err != nil {
		log.Fatal(err)
	}
}
