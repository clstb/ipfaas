package server

import (
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/openfaas/faas-provider/logs"
	faasdlogs "github.com/openfaas/faasd/pkg/logs"
	"github.com/openfaas/faasd/pkg/provider/handlers"
)

func (s *Server) routes() {
	readHandler := handlers.MakeReadHandler(s.containerd)
	deployHandler := handlers.MakeDeployHandler(s.containerd, s.cni, "", true) // TODO
	deleteHandler := handlers.MakeDeleteHandler(s.containerd, s.cni)
	updateHandler := handlers.MakeUpdateHandler(s.containerd, s.cni, "", true) // TODO
	replicaUpdateHandler := handlers.MakeReplicaUpdateHandler(s.containerd, s.cni)
	replicaReaderHandler := handlers.MakeReplicaReaderHandler(s.containerd)
	secretHandler := handlers.MakeSecretHandler(s.containerd.NamespaceService(), "") // TODO
	logHandler := logs.NewLogHandlerFunc(faasdlogs.New(), 0)                         // TODO
	namespacesLister := handlers.MakeNamespacesLister(s.containerd)
	functionHandler := s.FunctionHandler()

	s.Get("/system/functions", adaptor.HTTPHandlerFunc(readHandler))
	s.Post("/system/functions", adaptor.HTTPHandlerFunc(deployHandler))
	s.Delete("/system/functions", adaptor.HTTPHandlerFunc(deleteHandler))
	s.Put("/system/functions", adaptor.HTTPHandlerFunc(updateHandler))

	s.Post(`/system/scale-function/:name`, adaptor.HTTPHandlerFunc(replicaUpdateHandler))
	s.Get(`/system/function/:name`, adaptor.HTTPHandlerFunc(replicaReaderHandler))
	s.Get("/system/info", func(c *fiber.Ctx) error { return nil })

	s.All("/system/secrets", adaptor.HTTPHandlerFunc(secretHandler))
	s.Get("/system/logs", adaptor.HTTPHandlerFunc(logHandler))

	s.Get("/system/namespaces", adaptor.HTTPHandlerFunc(namespacesLister))

	s.All("/function/:name", functionHandler)
	s.All("/function/:name/*", functionHandler)

	s.Get("/healthz", func(c *fiber.Ctx) error { return nil })
}
