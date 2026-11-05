package logger

import "context"

type contextKey int

const (
	keyCorrelationID contextKey = iota
)

func ContextWithCorrID(ctx context.Context, corrID string) context.Context {
	return context.WithValue(ctx, keyCorrelationID, corrID)
}

func CorrIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(keyCorrelationID).(string)
	return userID, ok
}

func populateContext(ctx context.Context, attr []Attr) []Attr {
	if attr == nil {
		attr = []Attr{}
	}

	if val, exist := CorrIDFromContext(ctx); exist && val != "" {
		attr = append(attr, String("correlationID", val))
	}

	return attr
}
