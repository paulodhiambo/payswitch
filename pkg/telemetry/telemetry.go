package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var Tracer trace.Tracer

// InitTracer wires up an OTLP/gRPC trace exporter pointed at otlpEndpoint
// (e.g. "tempo:4317"). W3C TraceContext + Baggage propagators are installed
// globally so traces flow across HTTP and gRPC boundaries.
func InitTracer(ctx context.Context, otlpEndpoint, serviceName string) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
		resource.WithHost(),
		resource.WithProcessPID(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	Tracer = tp.Tracer(serviceName)

	return tp, nil
}

func InitLogger(serviceName string) *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}).WithAttrs([]slog.Attr{
		slog.String("service", serviceName),
	}))
	slog.SetDefault(logger)
	return logger
}

func SpanAttrs(paymentID, endToEndID, status string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("payment.id", paymentID),
		attribute.String("payment.end_to_end_id", endToEndID),
		attribute.String("payment.status", status),
	}
}
