package inbound

import (
	"context"

	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/app/proxyman/mux"
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/proxy"
	"v2ray.com/core/transport/internet/domainsocket"
)

type UnixInboundHandler struct {
	tag            string
	listenerHolder *domainsocket.Listener
	ctx            context.Context
	path           string
	proxy          proxy.Inbound
	mux            *mux.Server
	additional     *proxyman.UnixReceiverConfig
}

func (uih *UnixInboundHandler) Start() {
	var err error
	uih.listenerHolder, err = domainsocket.ListenDS(uih.ctx, uih.path)
	if err != nil {
		newError(err).AtError().WriteToLog()
	}
	err = uih.listenerHolder.LowerUP()
	if err != nil {
		newError(err).AtError().WriteToLog()
	}
	nchan := make(chan net.Conn, 2)
	err = uih.listenerHolder.UP(nchan, false)
	if err != nil {
		newError(err).AtError().WriteToLog()
	}
	go uih.progressTraffic(nchan)
	return
}
func (uih *UnixInboundHandler) progressTraffic(rece <-chan net.Conn) {

	for {
		conn, notclosed := <-rece
		if !notclosed {
			return
		}
		go func(conn net.Conn) {
			ctx, cancel := context.WithCancel(uih.ctx)
			if len(uih.tag) > 0 {
				ctx = proxy.ContextWithInboundTag(ctx, uih.tag)
			}
			if err := uih.proxy.Process(ctx, net.Network_TCP, conn, uih.mux); err != nil {
				newError("connection ends").Base(err).WriteToLog()
			}
			cancel()
			conn.Close()
		}(conn)
	}
}
func (uih *UnixInboundHandler) Close() {
	if uih.listenerHolder != nil {
		uih.listenerHolder.Down()
	} else {
		newError("Called UnixInboundHandler.Close while listenerHolder is nil").AtDebug().WriteToLog()
	}

}
func (uih *UnixInboundHandler) Tag() string {
	return uih.tag
}

func (uih *UnixInboundHandler) GetRandomInboundProxy() (interface{}, net.Port, int) {
	//It makes bo sense to support it
	return nil, 0, 0
}

func NewUnixInboundHandler(ctx context.Context, tag string, receiverConfig *proxyman.UnixReceiverConfig, proxyConfig interface{}) (*UnixInboundHandler, error) {
	rawProxy, err := common.CreateObject(ctx, proxyConfig)
	if err != nil {
		return nil, err
	}
	p, ok := rawProxy.(proxy.Inbound)
	if !ok {
		return nil, newError("not an inbound proxy.")
	}

	h := &UnixInboundHandler{
		proxy:      p,
		mux:        mux.NewServer(ctx),
		tag:        tag,
		ctx:        ctx,
		path:       receiverConfig.DomainSockSettings.GetPath(),
		additional: receiverConfig,
	}

	return h, nil

}
