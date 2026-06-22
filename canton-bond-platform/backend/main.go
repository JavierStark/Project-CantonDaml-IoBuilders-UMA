package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	providers, err := initOTel(ctx)
	if err != nil {
		log.Printf("[WARN] OTel init: %v (continuing without telemetry)", err)
	} else if providers != nil {
		defer func() {
			if err := providers.Shutdown(context.Background()); err != nil {
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

type otelProviders struct {
	traces  *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider
}

func (p *otelProviders) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	return errors.Join(
		shutdownProvider(ctx, p.metrics),
		shutdownProvider(ctx, p.traces),
	)
}

func shutdownProvider(ctx context.Context, provider interface{ Shutdown(context.Context) error }) error {
	if provider == nil {
		return nil
	}
	return provider.Shutdown(ctx)
}

func initOTel(ctx context.Context) (*otelProviders, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		return nil, nil
	}
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "backend"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(15*time.Second),
		)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &otelProviders{traces: tp, metrics: mp}, nil
}
