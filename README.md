# fallback

## problem

There is some problem in [apache/dubbo-go](https://github.com/apache/dubbo-go/) when using multiple services under grpc protocol. 

More detail, if you use multiple services under grpc protocol, only first service will be invoked successfully by consumers.

## reason

- Because services use the same address (or location in codes), so the `srv.Start(url)` will be executed only once. Refer to [openServer](https://github.com/apache/dubbo-go/blob/274718b650e71ee55a832203e87088f9762d4ff7/protocol/grpc/grpc_protocol.go#L70-L89).
- Then the grpc server on provider side will register the first service. Refer to [register in start](https://github.com/apache/dubbo-go/blob/274718b650e71ee55a832203e87088f9762d4ff7/protocol/grpc/server.go#L84-L109).

## how to fix

- Firstly, we cannot fix this problem directly because
  - the grpc server starts when the first service export and register
  - then other service cannot register after grpc server start
- Then we should find a way to register service after server start
  - [apache/dubbo](https://github.com/apache/dubbo) use a fallback method, refer to [DubboHandlerRegistry implements HandlerRegistry](https://github.com/apache/dubbo/blob/master/dubbo-rpc/dubbo-rpc-grpc/src/main/java/org/apache/dubbo/rpc/protocol/grpc/GrpcProtocol.java#L199)
  - we can use `UnknownServiceHandler` in [grpc/grpc-go/server.go](https://github.com/grpc/grpc-go/blob/bce1cded4b05db45e02a87b94b75fa5cb07a76a5/server.go), like [mwitkow/grpc-proxy](https://github.com/mwitkow/grpc-proxy)
  
## disadvantage

- Because we implement unary rpc by streaming rpc, we cannot use [unaryInt](https://github.com/grpc/grpc-go/blob/bce1cded4b05db45e02a87b94b75fa5cb07a76a5/server.go#L135).