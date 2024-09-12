package httpmiddleware

import (
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv/v1.20.0/httpconv"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var SkippedRoutes = []string{"/metrics"}

type tracingMiddleware struct {
	tracer     trace.Tracer
	next       http.Handler
	propagator propagation.TextMapPropagator
}

func NewTracingMiddleware(tracer trace.Tracer) func(next http.Handler) http.Handler {
	tMidd := &tracingMiddleware{
		tracer:     tracer,
		propagator: otel.GetTextMapPropagator(),
	}

	return func(next http.Handler) http.Handler {
		tMidd.next = next
		return tMidd
	}
}

func (tMidd *tracingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writerWrapper := &responseWriterWrapper{wrapped: w}

	route := chi.RouteContext(r.Context()).RoutePattern()
	if skipRoute(route, r) {
		tMidd.next.ServeHTTP(writerWrapper, r)
		return
	}

	ctx := tMidd.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	chi.NewRouteContext()

	spanName := r.Method + route

	attribs := httpconv.ServerRequest("jiboia", r)
	attribs = append(attribs, semconv.HTTPRoute(route))
	ctx, span := tMidd.tracer.Start(ctx, spanName, trace.WithAttributes(attribs...))
	defer span.End()

	r = r.WithContext(ctx)
	tMidd.next.ServeHTTP(writerWrapper, r)

	span.SetAttributes(semconv.HTTPResponseStatusCode(writerWrapper.statusCode))
	span.SetStatus(httpconv.ServerStatus(writerWrapper.statusCode))
}

func skipRoute(chiRoute string, r *http.Request) bool {
	if chiRoute != "" {
		return slices.Contains(SkippedRoutes, chiRoute)
	}

	return slices.Contains(SkippedRoutes, r.URL.Path)
}
