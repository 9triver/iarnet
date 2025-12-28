package provider

import (
	"context"

	common "github.com/9triver/iarnet/internal/proto/common"
)

type languageContextKey struct{}

// WithLanguage 将语言信息添加到 context 中
func WithLanguage(ctx context.Context, language common.Language) context.Context {
	return context.WithValue(ctx, languageContextKey{}, language)
}

// GetLanguageFromContext 从 context 中获取语言信息
func GetLanguageFromContext(ctx context.Context) (common.Language, bool) {
	language, ok := ctx.Value(languageContextKey{}).(common.Language)
	return language, ok
}
