package fallback

import (
	"reflect"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var logger = grpclog.Component("core")

type serviceInfo struct {
	serviceImpl interface{}
	methods     map[string]*grpc.MethodDesc
	streams     map[string]*grpc.StreamDesc
	mdata       interface{}
}

// refer to https://github.com/grpc/grpc-go/blob/bce1cded4b05db45e02a87b94b75fa5cb07a76a5/server.go
type RouteHandler struct {
	mu       sync.Mutex
	services map[string]*serviceInfo
}

func NewRouteHandler() *RouteHandler {
	return &RouteHandler{
		services: make(map[string]*serviceInfo),
	}
}

func (rh *RouteHandler) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			logger.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
	}
	rh.register(sd, ss)
}

func (rh *RouteHandler) register(sd *grpc.ServiceDesc, ss interface{}) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	if _, ok := rh.services[sd.ServiceName]; ok {
		logger.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*grpc.MethodDesc),
		streams:     make(map[string]*grpc.StreamDesc),
		mdata:       sd.Metadata,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	for i := range sd.Streams {
		d := &sd.Streams[i]
		info.streams[d.StreamName] = d
	}
	rh.services[sd.ServiceName] = info
}

func parseName(sm string) (string, string, error) {
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}

	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		return "", "", errors.Errorf("no / in %s", sm)
	}

	return sm[:pos], sm[pos+1:], nil
}

func (rh *RouteHandler) Handle(srv interface{}, stream grpc.ServerStream) error {
	method, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return errors.Errorf("cat no retrieve method because of no transport stream ctx in server stream")
	}

	service, method, err := parseName(method)
	if err != nil {
		return err
	}

	info, knownService := rh.services[service]
	if knownService {
		if md, ok := info.methods[method]; ok {
			return processUnaryRPC(stream, info, md)
		}
		if sd, ok := info.streams[method]; ok {
			return processStreamingRPC(stream, info, sd)
		}
	}

	if !knownService {
		return errors.Errorf("unknown service %v", service)
	} else {
		return errors.Errorf("unknown method %v for service %v", method, service)
	}
}

// refer to https://github.com/grpc/grpc-go/issues/1801#issuecomment-358379067
func processUnaryRPC(stream grpc.ServerStream, info *serviceInfo, md *grpc.MethodDesc) error {
	df := func(v interface{}) error {
		return stream.RecvMsg(v)
	}
	result, err := md.Handler(info.serviceImpl, stream.Context(), df, nil)
	if err != nil {
		return err
	}
	return stream.SendMsg(result)
}

func processStreamingRPC(stream grpc.ServerStream, info *serviceInfo, sd *grpc.StreamDesc) error {
	return sd.Handler(info.serviceImpl, stream)
}
