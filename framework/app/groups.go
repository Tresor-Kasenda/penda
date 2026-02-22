package app

// Group creates a route group with a shared path prefix and middleware.
func (a *App) Group(prefix string, middlewares ...Middleware) *Group {
	validateMiddlewares(middlewares)

	return &Group{
		app:         a,
		prefix:      normalizeGroupPrefix(prefix),
		middlewares: append([]Middleware(nil), middlewares...),
	}
}

// Use registers middleware in declaration order for this group.
func (g *Group) Use(middlewares ...Middleware) {
	validateMiddlewares(middlewares)
	g.middlewares = append(g.middlewares, middlewares...)
}

// Group creates a nested group inheriting parent middleware.
func (g *Group) Group(prefix string, middlewares ...Middleware) *Group {
	validateMiddlewares(middlewares)

	combined := append([]Middleware(nil), g.middlewares...)
	combined = append(combined, middlewares...)

	return &Group{
		app:         g.app,
		prefix:      joinPaths(g.prefix, prefix),
		middlewares: combined,
	}
}
