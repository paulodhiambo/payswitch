package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var Tracer trace.Tracer

// InitTracer wires up an OTLP/gRPC trace exporter pointed at otlpEndpoint
// (e.g. "jaeger:4317"). W3C TraceContext + Baggage propagators are installed
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

// InitLogger creates an slog.Logger that writes JSON to stdout and sends
// all log records to the OTel log exporter (which forwards to Uptrace).
// If otlpEndpoint is empty, only stdout is used.
// The caller must call Shutdown on the returned LoggerProvider when done.
func InitLogger(ctx context.Context, otlpEndpoint, serviceName string) (*slog.Logger, *sdklog.LoggerProvider, error) {
	var lp *sdklog.LoggerProvider

	if otlpEndpoint != "" {
		exp, err := otlploggrpc.New(ctx,
			otlploggrpc.WithEndpoint(otlpEndpoint),
			otlploggrpc.WithInsecure(),
		)
		if err != nil {
			return nil, nil, err
		}

		res, err := resource.New(ctx,
			resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
			resource.WithHost(),
			resource.WithProcessPID(),
		)
		if err != nil {
			return nil, nil, err
		}

		lp = sdklog.NewLoggerProvider(
			sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
			sdklog.WithResource(res),
		)
	}

	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}).WithAttrs([]slog.Attr{slog.String("service", serviceName)})

	var handler slog.Handler
	if lp != nil {
		otelHandler := otelslog.NewHandler(serviceName,
			otelslog.WithLoggerProvider(lp),
			otelslog.WithAttributes(
				attribute.String("service", serviceName),
			),
		)
		handler = slog.NewMultiHandler(stdoutHandler, otelHandler)
	} else {
		handler = stdoutHandler
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, lp, nil
}

func SpanAttrs(paymentID, endToEndID, status string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("payment.id", paymentID),
		attribute.String("payment.end_to_end_id", endToEndID),
		attribute.String("payment.status", status),
	}
}
