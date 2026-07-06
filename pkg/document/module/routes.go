package module

// Route registration for the document module.
//
// Transport: ConnectRPC handler wrapped by a Vanguard transcoder (Connect + gRPC
// + gRPC-Web on the standard path /goeland.v1.DocumentService/...).
//
// Standalone mode → RegisterRoutes(mux) builds a transcoder for this module only.
// Bundle mode     → VanguardServices() lets the caller build ONE shared transcoder.

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	connectvalidate "connectrpc.com/validate"
	"connectrpc.com/vanguard"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/authadapter"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

const maxRequestBodyBytes = 8 << 20 // 8 MiB (documents can carry larger metadata payloads)

// connectOption builds the standard interceptor chain: timeout → auth → proto validation.
func (m *Module) connectOption() connect.Option {
	return connect.WithInterceptors(
		core.NewTimeoutInterceptor(m.cfg.requestTimeout()),
		authadapter.NewInterceptor(m.deps.Verifier, m.deps.Logger),
		connectvalidate.NewInterceptor(),
	)
}

// VanguardServices returns the Vanguard services exposed by this module.
func (m *Module) VanguardServices() []*vanguard.Service {
	_, handler := goelandv1connect.NewDocumentServiceHandler(m.connect, m.connectOption())
	return []*vanguard.Service{
		vanguard.NewService(goelandv1connect.DocumentServiceName, handler),
	}
}

// ServiceNames returns the fully-qualified gRPC service names owned by this module.
func (m *Module) ServiceNames() []string {
	return []string{goelandv1connect.DocumentServiceName}
}

// RegisterRoutes mounts this module's transcoder on mux for standalone mode.
func (m *Module) RegisterRoutes(mux *http.ServeMux) error {
	transcoder, err := vanguard.NewTranscoder(m.VanguardServices())
	if err != nil {
		return fmt.Errorf("document module: build transcoder: %w", err)
	}
	for _, name := range m.ServiceNames() {
		mux.Handle("/"+name+"/", http.MaxBytesHandler(transcoder, maxRequestBodyBytes))
	}
	m.deps.Logger.Info("document module routes registered", "service", goelandv1connect.DocumentServiceName)
	return nil
}
