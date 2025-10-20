package httpmiddleware

import (
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
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
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	attribs := make([]attribute.KeyValue, 0, 8)
	attribs = append(attribs,
		attribute.String("http.method", r.Method),
		attribute.String("http.target", r.URL.RequestURI()),
		attribute.String("http.host", r.Host),
		attribute.String("http.scheme", scheme),
		attribute.String("http.user_agent", r.UserAgent()),
		attribute.String("http.flavor", r.Proto),
		attribute.String("http.route", route),
	)
	ctx, span := tMidd.tracer.Start(ctx, spanName, trace.WithAttributes(attribs...))
	defer span.End()

	r = r.WithContext(ctx)
	tMidd.next.ServeHTTP(writerWrapper, r)

	span.SetAttributes(semconv.HTTPResponseStatusCode(writerWrapper.statusCode))
	// Map HTTP status code to OpenTelemetry span status.
	if writerWrapper.statusCode >= 100 && writerWrapper.statusCode < 400 {
		span.SetStatus(codes.Ok, "")
	} else {
		span.SetStatus(codes.Error, http.StatusText(writerWrapper.statusCode))
	}
}

func skipRoute(chiRoute string, r *http.Request) bool {
	if chiRoute != "" {
		return slices.Contains(SkippedRoutes, chiRoute)
	}

	return slices.Contains(SkippedRoutes, r.URL.Path)
}
