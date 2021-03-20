package fallback

import (
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"strings"
)

type serviceInfo struct {
	serviceImpl interface{}
	methods     map[string]*grpc.MethodDesc
	streams     map[string]*grpc.StreamDesc
	mdata       interface{}
}

type RouteHandler struct {
	services map[string]*serviceInfo
}

func NewRouteHandler() *RouteHandler {
	return &RouteHandler{
		services: make(map[string]*serviceInfo),
	}
}

func (rh *RouteHandler) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	// TODO: type implement check
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
