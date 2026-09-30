package main

import (
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// The Vanguard transcoder answers a query parameter that names no request field
// with UNKNOWN / HTTP 500 (connectrpc.com/vanguard v0.3.0 and v0.4.0), although it
// is a client error. The middleware below checks the query parameters of the REST
// bindings against their request message first and answers 400 INVALID_ARGUMENT
// (GLD-043). Invalid values are left to the transcoder, which already answers 400.

// restRoute is one google.api.http binding: its HTTP method, its path template
// split in segments and the request message it fills.
type restRoute struct {
	method   string
	segments []string
	request  protoreflect.MessageDescriptor
}

// restRoutes reads the REST bindings of the named services from the registered
// descriptors (the generated code registers them).
func restRoutes(serviceNames []string) ([]restRoute, error) {
	var routes []restRoute
	for _, name := range serviceNames {
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			return nil, fmt.Errorf("find service %s: %w", name, err)
		}
		service, ok := desc.(protoreflect.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("%s is not a service", name)
		}
		for i := range service.Methods().Len() {
			routes = append(routes, methodRoutes(service.Methods().Get(i))...)
		}
	}
	return routes, nil
}

// methodRoutes returns the REST bindings of one RPC (none without google.api.http).
func methodRoutes(method protoreflect.MethodDescriptor) []restRoute {
	rule, _ := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
	if rule == nil {
		return nil
	}
	var routes []restRoute
	for _, binding := range append([]*annotations.HttpRule{rule}, rule.GetAdditionalBindings()...) {
		if verb, path := httpRulePattern(binding); path != "" {
			routes = append(routes, restRoute{method: verb, segments: splitPath(path), request: method.Input()})
		}
	}
	return routes
}

// httpRulePattern returns the HTTP method and path template of a binding.
func httpRulePattern(rule *annotations.HttpRule) (string, string) {
	switch pattern := rule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		return http.MethodGet, pattern.Get
	case *annotations.HttpRule_Post:
		return http.MethodPost, pattern.Post
	case *annotations.HttpRule_Put:
		return http.MethodPut, pattern.Put
	case *annotations.HttpRule_Patch:
		return http.MethodPatch, pattern.Patch
	case *annotations.HttpRule_Delete:
		return http.MethodDelete, pattern.Delete
	case *annotations.HttpRule_Custom:
		return pattern.Custom.GetKind(), pattern.Custom.GetPath()
	}
	return "", ""
}

// splitPath splits a URL path or path template into its segments.
func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

// matches reports whether a request path fits the route's template: literal
// segments are equal, a {variable} takes one non-empty segment and a
// {variable=**} template takes the rest.
func (r restRoute) matches(method string, segments []string) bool {
	if method != r.method {
		return false
	}
	for i, tmpl := range r.segments {
		if strings.HasPrefix(tmpl, "{") && strings.Contains(tmpl, "**") {
			return len(segments) > i
		}
		if i >= len(segments) {
			return false
		}
		if strings.HasPrefix(tmpl, "{") {
			if segments[i] == "" {
				return false
			}
			continue
		}
		if tmpl != segments[i] {
			return false
		}
	}
	return len(segments) == len(r.segments)
}

// literals counts the literal segments of the route's template: among the
// routes matching a path, the one with most literals is the one served
// ("/api/cases/search" before "/api/cases/{id}").
func (r restRoute) literals() int {
	n := 0
	for _, tmpl := range r.segments {
		if !strings.HasPrefix(tmpl, "{") {
			n++
		}
	}
	return n
}

// routeFor returns the most specific route matching the request, or nil.
func routeFor(routes []restRoute, method string, segments []string) *restRoute {
	var best *restRoute
	for i := range routes {
		if routes[i].matches(method, segments) && (best == nil || routes[i].literals() > best.literals()) {
			best = &routes[i]
		}
	}
	return best
}

// knownFieldPath reports whether a dotted query parameter name (JSON or proto
// field names, as Vanguard accepts) resolves to a field of msg.
func knownFieldPath(msg protoreflect.MessageDescriptor, path string) bool {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		field := msg.Fields().ByJSONName(part)
		if field == nil {
			field = msg.Fields().ByName(protoreflect.Name(part))
		}
		if field == nil {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		if field.Cardinality() == protoreflect.Repeated || field.Message() == nil {
			return false
		}
		msg = field.Message()
	}
	return false
}

// unknownQueryParamsMiddleware answers 400 INVALID_ARGUMENT, in the Connect
// error shape the REST clients already read, when a query parameter of a known
// REST route names no field of its request message.
func unknownQueryParamsMiddleware(routes []restRoute, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if len(query) == 0 {
			next.ServeHTTP(writer, request)
			return
		}
		if route := routeFor(routes, request.Method, splitPath(request.URL.Path)); route != nil {
			for name := range query {
				if !knownFieldPath(route.request, name) {
					writeJSON(writer, http.StatusBadRequest, map[string]any{
						"code":    3, // INVALID_ARGUMENT
						"message": fmt.Sprintf("unknown query parameter %q for %s", name, route.request.FullName()),
						"details": []any{},
					})
					return
				}
			}
		}
		next.ServeHTTP(writer, request)
	})
}
