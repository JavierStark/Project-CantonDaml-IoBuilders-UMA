package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tp, err := initOTel(ctx)
	if err != nil {
		log.Printf("[WARN] OTel init: %v (continuing without tracing)", err)
	} else {
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				log.Printf("[WARN] OTel shutdown: %v", err)
			}
		}()
	}

	cfg, err := Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	s, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("server init error: %v", err)
	}
	e := s.Router()

	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	log.Printf("bond API listening on %s", addr)

	log.Printf("ledger transport: %s", cfg.LedgerTransport)
	for _, p := range cfg.Participants {
		if cfg.LedgerTransport == "grpc" {
			log.Printf("  participant %s -> %s (grpc), fallback http %s [%s]", p.Name, p.GRPCURL, p.URL, p.Parties)
		} else {
			log.Printf("  participant %s -> %s (http) [%s]", p.Name, p.URL, p.Parties)
		}
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down...")
		e.Close()
	}()

	e.Logger.Fatal(e.Start(addr))
}

func initOTel(ctx context.Context) (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return nil, nil
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("backend"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}
