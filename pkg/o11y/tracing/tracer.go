package tracing

import (
	"context"

	"github.com/jademcosta/jiboia/pkg/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func NewNoopTracer() trace.Tracer {
	return noop.NewTracerProvider().Tracer("github.com/jademcosta/jiboia")
}

func NewTracer(conf config.Config) (trace.Tracer, func(context.Context) error) {
	bsp := sdktrace.NewBatchSpanProcessor(newExporter())
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(buildResource()),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	tracer := tracerProvider.Tracer("github.com/jademcosta/jiboia")

	return tracer, tracerProvider.Shutdown
}

func newExporter() sdktrace.SpanExporter {
	ctx := context.Background() //TODO: use a real one
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		panic(err)
	}

	return exporter
}

func buildResource() *resource.Resource {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("jiboia"),
		),
	)

	if err != nil {
		panic(err)
	}

	return res
}
