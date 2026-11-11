// Package obs sets up OpenTelemetry tracing/metrics and JSON slog logging.
package obs

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"nongnooch/platform/pkg/chassis/config"
)

// Setup installs global tracer/meter providers, W3C propagation and a JSON slog
// default logger that adds trace_id. With no OTLP endpoint, spans are still
// created (valid ids, so trace ids propagate) but nothing is exported.
func Setup(ctx context.Context, cfg config.Base) (func(context.Context) error, error) {
	res, err := resource.Merge(resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(cfg.ServiceName)))
	if err != nil {
		return nil, err
	}
	tpOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	var mp *sdkmetric.MeterProvider
	if cfg.OTLPEndpoint != "" {
		te, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, err
		}
		tpOpts = append(tpOpts, sdktrace.WithBatcher(te))
		me, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, err
		}
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)))
		otel.SetMeterProvider(mp)
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	slog.SetDefault(slog.New(traceHandler{slog.NewJSONHandler(os.Stdout, nil)}).With("service", cfg.ServiceName))

	return func(ctx context.Context) error {
		err := tp.Shutdown(ctx)
		if mp != nil {
			err = errors.Join(err, mp.Shutdown(ctx))
		}
		return err
	}, nil
}

type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(a []slog.Attr) slog.Handler { return traceHandler{h.Handler.WithAttrs(a)} }
func (h traceHandler) WithGroup(n string) slog.Handler      { return traceHandler{h.Handler.WithGroup(n)} }
